// Security-critical subprocess runner for the Noli CLI.
//
// Frozen rules (PLANS.md Phase 8):
// - spawn, never exec; shell: false; separate argument array;
// - only the five allowlisted read operations;
// - the executable path comes from host configuration, never from the agent;
// - knowledge roots resolve to real paths and must stay inside the
//   repository (symlink escapes rejected);
// - stdout and stderr are bounded while streaming; the process is killed
//   immediately on timeout or overflow;
// - invalid JSON and non-zero exits produce useful typed errors.
import { spawn } from "node:child_process";
import { existsSync, realpathSync } from "node:fs";
import path from "node:path";

export type Operation = "status" | "search" | "retrieve" | "get" | "graph";

const OPERATIONS: ReadonlySet<string> = new Set([
  "status",
  "search",
  "retrieve",
  "get",
  "graph",
]);

const DIRECTIONS: ReadonlySet<string> = new Set(["outgoing", "incoming", "both"]);

export const DEFAULT_TIMEOUT_MS = 10_000;
export const DEFAULT_MAX_OUTPUT_BYTES = 2 * 1024 * 1024;

export class NoliError extends Error {
  readonly code: string;
  readonly exitCode: number | null;

  constructor(message: string, code: string, exitCode: number | null = null) {
    super(message);
    this.name = "NoliError";
    this.code = code;
    this.exitCode = exitCode;
  }
}

export interface RunnerOptions {
  /** Absolute path to the noli binary; fixed by host configuration. */
  binaryPath: string;
  /** Repository root; every knowledge root must stay inside it. */
  repositoryRoot: string;
  timeoutMs?: number;
  maxOutputBytes?: number;
  /** Pi tool cancellation signal; abort kills the subprocess immediately. */
  signal?: AbortSignal;
}

export interface NoliRequest {
  operation: Operation;
  /** Knowledge root, relative to the repository root. */
  root: string;
  query?: string;
  id?: string;
  types?: string[];
  limit?: number;
  searchLimit?: number;
  maxHops?: number;
  maxDocuments?: number;
  maxCharacters?: number;
  direction?: string;
}

function requireCleanString(value: string, field: string): string {
  if (value.includes("\0")) {
    throw new NoliError(`${field} contains a NUL byte`, "INVALID_ARGUMENT");
  }
  return value;
}

function requireBound(value: number | undefined, field: string): number | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!Number.isInteger(value) || value < 0) {
    throw new NoliError(`${field} must be a non-negative integer`, "INVALID_ARGUMENT");
  }
  return value;
}

/** Resolves the knowledge root and rejects anything escaping the repository. */
export function resolveKnowledgeRoot(repositoryRoot: string, root: string): string {
  requireCleanString(root, "root");
  if (root.trim() === "") {
    throw new NoliError("root is required", "INVALID_ARGUMENT");
  }
  let repositoryReal: string;
  try {
    repositoryReal = realpathSync(repositoryRoot);
  } catch {
    throw new NoliError(`repository root not found: ${repositoryRoot}`, "KNOWLEDGE_NOT_FOUND");
  }
  // The primary Noli sentinel wins. The former OKF sentinel is read only when
  // no primary Noli configuration or state directory exists.
  const noliDisabled = existsSync(path.join(repositoryReal, ".noli", "disabled"));
  const hasNoliNamespace =
    existsSync(path.join(repositoryReal, "noli.yaml")) ||
    existsSync(path.join(repositoryReal, ".noli"));
  const legacyDisabled =
    !hasNoliNamespace && existsSync(path.join(repositoryReal, ".okf", "disabled"));
  if (noliDisabled || legacyDisabled) {
    const sentinel = noliDisabled ? ".noli/disabled" : ".okf/disabled (legacy)";
    throw new NoliError(
      `Noli is disabled for this repository (${sentinel} exists); work without it`,
      "NOLI_DISABLED",
    );
  }
  const candidate = path.resolve(repositoryReal, root);
  let real: string;
  try {
    real = realpathSync(candidate);
  } catch {
    throw new NoliError(`knowledge root not found: ${root}`, "KNOWLEDGE_NOT_FOUND");
  }
  const prefix = repositoryReal.endsWith(path.sep) ? repositoryReal : repositoryReal + path.sep;
  if (real !== repositoryReal && !real.startsWith(prefix)) {
    throw new NoliError(
      `knowledge root ${root} escapes the repository after symlink resolution`,
      "UNSAFE_PATH",
    );
  }
  return real;
}

/** Builds the argument array for one allowlisted operation. */
export function buildArguments(request: NoliRequest, resolvedRoot: string): string[] {
  const args: string[] = [request.operation, "--root", resolvedRoot, "--format", "json"];
  const pushTypes = (): void => {
    if (request.types && request.types.length > 0) {
      for (const type of request.types) {
        requireCleanString(type, "types");
      }
      args.push("--types", request.types.join(","));
    }
  };
  const pushDirection = (): void => {
    if (request.direction !== undefined) {
      if (!DIRECTIONS.has(request.direction)) {
        throw new NoliError(
          `direction must be outgoing, incoming, or both; got ${request.direction}`,
          "INVALID_ARGUMENT",
        );
      }
      args.push("--direction", request.direction);
    }
  };
  const pushBound = (flag: string, value: number | undefined, field: string): void => {
    const bounded = requireBound(value, field);
    if (bounded !== undefined) {
      args.push(flag, String(bounded));
    }
  };

  switch (request.operation) {
    case "status":
      break;
    case "search": {
      if (!request.query || request.query.trim() === "") {
        throw new NoliError("query is required for search", "INVALID_ARGUMENT");
      }
      args.push("--query", requireCleanString(request.query, "query"));
      pushBound("--limit", request.limit, "limit");
      pushTypes();
      break;
    }
    case "retrieve": {
      if (!request.query || request.query.trim() === "") {
        throw new NoliError("query is required for retrieve", "INVALID_ARGUMENT");
      }
      args.push("--query", requireCleanString(request.query, "query"));
      pushBound("--search-limit", request.searchLimit, "searchLimit");
      pushBound("--max-hops", request.maxHops, "maxHops");
      pushBound("--max-documents", request.maxDocuments, "maxDocuments");
      pushBound("--max-characters", request.maxCharacters, "maxCharacters");
      pushDirection();
      pushTypes();
      break;
    }
    case "get":
    case "graph": {
      if (!request.id || request.id.trim() === "") {
        throw new NoliError(`id is required for ${request.operation}`, "INVALID_ARGUMENT");
      }
      args.push("--id", requireCleanString(request.id, "id"));
      if (request.operation === "graph") {
        pushDirection();
        pushBound("--max-hops", request.maxHops, "maxHops");
      }
      break;
    }
  }
  return args;
}

/**
 * Runs one allowlisted okf operation and returns the parsed `data` payload.
 * Rejects with NoliError on any protocol, containment, or process failure.
 */
export function runNoli(request: NoliRequest, options: RunnerOptions): Promise<unknown> {
  if (!OPERATIONS.has(request.operation)) {
    return Promise.reject(
      new NoliError(`operation ${String(request.operation)} is not allowed`, "INVALID_ARGUMENT"),
    );
  }
  let args: string[];
  try {
    const root = resolveKnowledgeRoot(options.repositoryRoot, request.root);
    args = buildArguments(request, root);
  } catch (error) {
    return Promise.reject(error);
  }
  const timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  const maxOutputBytes = options.maxOutputBytes ?? DEFAULT_MAX_OUTPUT_BYTES;
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) {
    return Promise.reject(
      new NoliError("timeoutMs must be a positive number", "INVALID_ARGUMENT"),
    );
  }
  if (!Number.isSafeInteger(maxOutputBytes) || maxOutputBytes <= 0) {
    return Promise.reject(
      new NoliError("maxOutputBytes must be a positive integer", "INVALID_ARGUMENT"),
    );
  }
  if (options.signal?.aborted) {
    return Promise.reject(
      new NoliError("okf execution was aborted before launch", "ABORTED"),
    );
  }

  return new Promise((resolve, reject) => {
    const child = spawn(options.binaryPath, args, {
      shell: false,
      stdio: ["ignore", "pipe", "pipe"],
    });
    const stdoutChunks: Buffer[] = [];
    let stdoutBytes = 0;
    let stderrBytes = 0;
    let killedFor: "abort" | "timeout" | "output-limit" | null = null;
    let settled = false;
    let timer: ReturnType<typeof setTimeout>;
    const onAbort = (): void => {
      if (settled) return;
      killedFor = "abort";
      killChild();
      fail(new NoliError("okf execution was aborted and the process was killed", "ABORTED"));
    };
    const cleanup = (): void => {
      clearTimeout(timer);
      options.signal?.removeEventListener("abort", onAbort);
    };
    const fail = (error: NoliError): void => {
      if (!settled) {
        settled = true;
        cleanup();
        reject(error);
      }
    };
    // Settle immediately when killing: a grandchild process could keep the
    // stdio pipes open past the SIGKILL, delaying the "close" event and
    // pinning the event loop. Destroying the streams releases the pipes.
    const killChild = (): void => {
      child.kill("SIGKILL");
      child.stdout.destroy();
      child.stderr.destroy();
    };
    timer = setTimeout(() => {
      killedFor = "timeout";
      killChild();
      fail(new NoliError(`okf timed out after ${timeoutMs}ms and was killed`, "TIMEOUT"));
    }, timeoutMs);
    options.signal?.addEventListener("abort", onAbort, { once: true });
    const killForOutputLimit = (): void => {
      killedFor = "output-limit";
      killChild();
      fail(
        new NoliError(`okf exceeded the ${maxOutputBytes} byte output limit and was killed`, "OUTPUT_LIMIT"),
      );
    };

    child.stdout.on("data", (chunk: Buffer) => {
      stdoutBytes += chunk.length;
      if (stdoutBytes > maxOutputBytes) {
        killForOutputLimit();
        return;
      }
      stdoutChunks.push(Buffer.from(chunk));
    });
    child.stderr.on("data", (chunk: Buffer) => {
      stderrBytes += chunk.length;
      if (stderrBytes > maxOutputBytes) {
        killForOutputLimit();
      }
      // stderr content is diagnostics only; it is never surfaced raw.
    });
    child.on("error", (error) => {
      fail(new NoliError(`failed to launch okf: ${error.message}`, "INTERNAL_ERROR"));
    });
    child.on("close", (exitCode) => {
      cleanup();
      if (killedFor !== null || settled) {
        return; // already settled by the kill path
      }
      const stdout = Buffer.concat(stdoutChunks, stdoutBytes).toString("utf8");
      let envelope: {
        ok?: boolean;
        command?: string;
        version?: number;
        data?: unknown;
        error?: { code?: string; message?: string };
      };
      try {
        envelope = JSON.parse(stdout) as typeof envelope;
      } catch {
        return fail(
          new NoliError(`okf returned invalid JSON (exit ${exitCode})`, "INVALID_JSON", exitCode),
        );
      }
      if (envelope.version !== 1 || envelope.command !== request.operation) {
        return fail(
          new NoliError(
            `okf returned an invalid protocol envelope (command ${String(envelope.command)}, version ${String(envelope.version)})`,
            "INVALID_PROTOCOL",
            exitCode,
          ),
        );
      }
      if (exitCode !== 0 || envelope.ok !== true) {
        const detail = envelope.error;
        return fail(
          new NoliError(
            detail?.message ?? `okf failed with exit code ${exitCode}`,
            detail?.code ?? "INTERNAL_ERROR",
            exitCode,
          ),
        );
      }
      if (stderrBytes !== 0) {
        return fail(
          new NoliError(
            "okf emitted stderr during a successful JSON command",
            "INVALID_PROTOCOL",
            exitCode,
          ),
        );
      }
      settled = true;
      resolve(envelope.data);
    });
  });
}
