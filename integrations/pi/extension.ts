// Noli Pi extension exposing five bounded, read-only OKF tools.
//
// The shapes below follow Pi 0.78's ExtensionAPI/ToolDefinition contract while
// remaining structural, so this adapter does not need to bundle Pi itself.
// Pi's loader supplies the ExtensionAPI and validates the JSON schemas.
import { spawn } from "node:child_process";
import {
  accessSync,
  constants,
  existsSync,
  mkdirSync,
  readFileSync,
  realpathSync,
  writeFileSync,
} from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { NoliError, runNoli, type NoliRequest, type Operation, type RunnerOptions } from "./runner.ts";

export interface PiToolContext {
  cwd: string;
}

export interface PiToolResult {
  content: Array<{ type: "text"; text: string }>;
  details: { operation: Operation; data: unknown };
}

export interface NoliToolDefinition {
  name: string;
  label: string;
  description: string;
  promptSnippet: string;
  parameters: Record<string, unknown>;
  executionMode: "parallel";
  execute(
    toolCallId: string,
    input: Record<string, unknown>,
    signal: AbortSignal | undefined,
    onUpdate: unknown,
    context: PiToolContext,
  ): Promise<PiToolResult>;
}

/** Fired by Pi when a session is started, loaded, or reloaded. */
export interface PiSessionStartEvent {
  type: "session_start";
  reason: "startup" | "reload" | "new" | "resume" | "fork";
}

/** The dialog and notification subset of Pi's extension UI context. */
export interface PiSessionUI {
  select(title: string, options: string[]): Promise<string | undefined>;
  notify(message: string, type?: "info" | "warning" | "error"): void;
}

/** The verified subset of Pi's event handler context. */
export interface PiSessionContext {
  cwd: string;
  hasUI: boolean;
  ui: PiSessionUI;
}

/** The verified subset of Pi's ExtensionAPI used by this extension. */
export interface PiExtensionAPI {
  registerTool(tool: NoliToolDefinition): void;
  on?(
    event: "session_start",
    handler: (event: PiSessionStartEvent, ctx: PiSessionContext) => void,
  ): void;
}

interface ToolSpec {
  operation: Operation;
  name: string;
  label: string;
  description: string;
  parameters: Record<string, unknown>;
}

type OptionsResolver = (context: PiToolContext) => RunnerOptions;

const ROOT_PARAMETER = {
  root: {
    type: "string",
    minLength: 1,
    description: "Knowledge root directory, relative to the repository root (usually 'knowledge').",
  },
} as const;

const TOOL_SPECS: ToolSpec[] = [
  {
    operation: "status",
    name: "noli_status",
    label: "Noli Status",
    description: "Summarize the project knowledge bundle: document counts, types, and bundle checksum.",
    parameters: { type: "object", properties: { ...ROOT_PARAMETER }, required: ["root"] },
  },
  {
    operation: "search",
    name: "noli_search",
    label: "Noli Search",
    description: "Keyword-search project knowledge documents; returns ranked hits with integer scores.",
    parameters: {
      type: "object",
      properties: {
        ...ROOT_PARAMETER,
        query: { type: "string", minLength: 1, description: "Search query." },
        limit: { type: "integer", minimum: 0, description: "Maximum results (default 10)." },
        types: { type: "array", items: { type: "string" }, description: "Restrict to these concept types." },
      },
      required: ["root", "query"],
    },
  },
  {
    operation: "retrieve",
    name: "noli_retrieve",
    label: "Noli Retrieve",
    description:
      "Retrieve a bounded, source-traceable knowledge context for a coding task (search seeds plus graph expansion).",
    parameters: {
      type: "object",
      properties: {
        ...ROOT_PARAMETER,
        query: { type: "string", minLength: 1, description: "Task description." },
        types: { type: "array", items: { type: "string" } },
        searchLimit: { type: "integer", minimum: 0 },
        maxHops: { type: "integer", minimum: 0 },
        maxDocuments: { type: "integer", minimum: 0 },
        maxCharacters: { type: "integer", minimum: 0 },
        direction: { type: "string", enum: ["outgoing", "incoming", "both"] },
      },
      required: ["root", "query"],
    },
  },
  {
    operation: "get",
    name: "noli_get",
    label: "Noli Get",
    description: "Fetch one knowledge document by ID, including metadata, typed links, and body.",
    parameters: {
      type: "object",
      properties: {
        ...ROOT_PARAMETER,
        id: { type: "string", minLength: 1, description: "Document ID, for example rules/complete-task." },
      },
      required: ["root", "id"],
    },
  },
  {
    operation: "graph",
    name: "noli_graph",
    label: "Noli Graph",
    description: "Show the bounded relationship neighborhood of one knowledge document.",
    parameters: {
      type: "object",
      properties: {
        ...ROOT_PARAMETER,
        id: { type: "string", minLength: 1 },
        direction: { type: "string", enum: ["outgoing", "incoming", "both"] },
        maxHops: { type: "integer", minimum: 0 },
      },
      required: ["root", "id"],
    },
  },
];

function requestFor(spec: ToolSpec, input: Record<string, unknown>): NoliRequest {
  return {
    operation: spec.operation,
    root: String(input.root ?? ""),
    query: input.query === undefined ? undefined : String(input.query),
    id: input.id === undefined ? undefined : String(input.id),
    types: Array.isArray(input.types) ? input.types.map(String) : undefined,
    limit: input.limit === undefined ? undefined : Number(input.limit),
    searchLimit: input.searchLimit === undefined ? undefined : Number(input.searchLimit),
    maxHops: input.maxHops === undefined ? undefined : Number(input.maxHops),
    maxDocuments: input.maxDocuments === undefined ? undefined : Number(input.maxDocuments),
    maxCharacters: input.maxCharacters === undefined ? undefined : Number(input.maxCharacters),
    direction: input.direction === undefined ? undefined : String(input.direction),
  };
}

function toolsWithOptions(resolveOptions: OptionsResolver): NoliToolDefinition[] {
  return TOOL_SPECS.map((spec) => ({
    name: spec.name,
    label: spec.label,
    description: spec.description,
    promptSnippet: `${spec.name}: ${spec.description}`,
    parameters: spec.parameters,
    executionMode: "parallel",
    async execute(_toolCallId, input, signal, _onUpdate, context) {
      const configured = resolveOptions(context);
      const data = await runNoli(requestFor(spec, input), {
        ...configured,
        signal: signal ?? configured.signal,
      });
      return {
        content: [{ type: "text", text: JSON.stringify(data) }],
        details: { operation: spec.operation, data },
      };
    },
  }));
}

/** Builds the five allowlisted tools with fixed host configuration. */
export function createNoliTools(options: RunnerOptions): NoliToolDefinition[] {
  return toolsWithOptions(() => options);
}

/** Registers all five tools with fixed host configuration. */
export function register(pi: PiExtensionAPI, options: RunnerOptions): void {
  for (const tool of createNoliTools(options)) {
    pi.registerTool(tool);
  }
}

function configuredBinary(): string {
  const primary = process.env.NOLI_BINARY_PATH?.trim();
  const legacy = process.env.OKF_BINARY_PATH?.trim();
  const explicit = primary || legacy;
  const variable = primary ? "NOLI_BINARY_PATH" : "OKF_BINARY_PATH";
  if (explicit) {
    if (!path.isAbsolute(explicit)) {
      throw new NoliError(`${variable} must be absolute`, "INTERNAL_ERROR");
    }
    try {
      accessSync(explicit, constants.X_OK);
      return realpathSync(explicit);
    } catch {
      throw new NoliError(`${variable} is not executable: ${explicit}`, "INTERNAL_ERROR");
    }
  }
  for (const name of ["noli", "okf"]) {
    const executable = process.platform === "win32" ? `${name}.exe` : name;
    for (const entry of (process.env.PATH ?? "").split(path.delimiter)) {
      if (!entry) continue;
      const candidate = path.resolve(entry, executable);
      try {
        accessSync(candidate, constants.X_OK);
        return realpathSync(candidate);
      } catch {
        // Continue through PATH without invoking a shell.
      }
    }
  }
  throw new NoliError(
    "noli executable was not found; install it on PATH or set NOLI_BINARY_PATH",
    "INTERNAL_ERROR",
  );
}

function configuredRepository(cwd: string): string {
  const primary = process.env.NOLI_REPOSITORY_ROOT?.trim();
  const legacy = process.env.OKF_REPOSITORY_ROOT?.trim();
  const explicit = primary || legacy;
  const variable = primary ? "NOLI_REPOSITORY_ROOT" : "OKF_REPOSITORY_ROOT";
  if (explicit) {
    if (!path.isAbsolute(explicit)) {
      throw new NoliError(`${variable} must be absolute`, "INTERNAL_ERROR");
    }
    try {
      return realpathSync(explicit);
    } catch {
      throw new NoliError(`${variable} does not exist: ${explicit}`, "INTERNAL_ERROR");
    }
  }

  let current: string;
  try {
    current = realpathSync(cwd);
  } catch {
    throw new NoliError(`Pi working directory does not exist: ${cwd}`, "INTERNAL_ERROR");
  }
  const fallback = current;
  while (true) {
    if (
      existsSync(path.join(current, "noli.yaml")) ||
      existsSync(path.join(current, "okf.yaml")) ||
      existsSync(path.join(current, "knowledge")) ||
      existsSync(path.join(current, ".git"))
    ) {
      return current;
    }
    const parent = path.dirname(current);
    if (parent === current) return fallback;
    current = parent;
  }
}

// ---- First-run choice on session start ----------------------------------
//
// Implements rules/agent-global-first-run-choice at the Pi UI layer: in a
// repository with no Noli state, ask the developer exactly one Yes/No
// question at startup. The session_start handler itself never awaits — Pi
// awaits session_start handlers during initialization, so the dialog and
// the bootstrap run as a detached promise while startup completes.

/** Repository Noli state per rules/agent-global-first-run-choice. */
export type NoliState = "enabled" | "disabled" | "undecided";

export const FIRST_RUN_QUESTION =
  "This project has Noli installed but no project knowledge base yet. " +
  "Initialize one so I can retrieve grounded project knowledge?";
export const FIRST_RUN_YES = "Yes — initialize a knowledge base";
export const FIRST_RUN_NO = "No — disable Noli for this repository";

/** Options for tests and embedders; production uses the defaults. */
export interface FirstRunOptions {
  binaryPath?: string;
  starterConfigPath?: string;
  starterConceptsPath?: string;
  timeoutMs?: number;
}

/**
 * Resolves the repository's Noli state. The Noli namespace always wins;
 * the former OKF names are deprecated migration fallbacks only.
 */
export function resolveNoliState(directory: string): NoliState {
  if (
    existsSync(path.join(directory, "noli.yaml")) ||
    existsSync(path.join(directory, "knowledge"))
  ) {
    return "enabled";
  }
  if (existsSync(path.join(directory, ".noli", "disabled"))) {
    return "disabled";
  }
  if (existsSync(path.join(directory, "okf.yaml"))) {
    return "enabled";
  }
  if (existsSync(path.join(directory, ".okf", "disabled"))) {
    return "disabled";
  }
  return "undecided";
}

/**
 * The complete first-run flow. Synchronous guards keep the startup cost of
 * decided repositories at four existsSync calls; everything slower happens
 * after this function's first await. Never rejects: failures surface as UI
 * notifications so a detached invocation cannot become an unhandled
 * rejection.
 */
export async function runSessionStart(
  event: PiSessionStartEvent,
  context: PiSessionContext,
  options: FirstRunOptions = {},
): Promise<void> {
  try {
    if (event.reason !== "startup" && event.reason !== "new") return;
    if (!context.hasUI) return;
    if (resolveNoliState(context.cwd) !== "undecided") return;

    let binaryPath: string;
    try {
      binaryPath = options.binaryPath ?? configuredBinary();
    } catch {
      context.ui.notify(
        "Noli is installed for Pi but the noli executable was not found; " +
          "install it on PATH or set NOLI_BINARY_PATH.",
        "warning",
      );
      return;
    }

    const choice = await context.ui.select(FIRST_RUN_QUESTION, [FIRST_RUN_YES, FIRST_RUN_NO]);
    if (choice === FIRST_RUN_NO) {
      mkdirSync(path.join(context.cwd, ".noli"), { recursive: true });
      writeFileSync(path.join(context.cwd, ".noli", "disabled"), "developer opted out\n");
      context.ui.notify("Noli disabled for this repository (.noli/disabled created).", "info");
      return;
    }
    if (choice !== FIRST_RUN_YES) {
      return; // Dismissed is not a decision; ask again next session.
    }
    await bootstrapKnowledgeBase(context, binaryPath, options);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    try {
      context.ui.notify(`Noli first-run setup failed: ${message}`, "error");
    } catch {
      // The UI is gone (session replaced); the next session asks again.
    }
  }
}

/**
 * Bootstrap per the shared skill: copy the starter configuration and
 * concepts, then generate and validate the bundle. Existing files are never
 * overwritten.
 */
async function bootstrapKnowledgeBase(
  context: PiSessionContext,
  binaryPath: string,
  options: FirstRunOptions,
): Promise<void> {
  const configTemplate = readFileSync(
    starterPath("noli-starter.yaml", options.starterConfigPath),
    "utf8",
  );
  const conceptsTemplate = readFileSync(
    starterPath("noli-starter-concepts.yaml", options.starterConceptsPath),
    "utf8",
  );

  const configPath = path.join(context.cwd, "noli.yaml");
  if (!existsSync(configPath)) {
    const projectName = (path.basename(context.cwd) || "My Project")
      .replaceAll("\\", "\\\\")
      .replaceAll('"', '\\"');
    writeFileSync(configPath, configTemplate.replace(/^ {2}name: .*$/m, `  name: "${projectName}"`));
  }
  const conceptsPath = path.join(context.cwd, ".noli", "concepts.yaml");
  if (!existsSync(conceptsPath)) {
    mkdirSync(path.dirname(conceptsPath), { recursive: true });
    writeFileSync(conceptsPath, conceptsTemplate);
  }

  await runNoliArgv(
    binaryPath,
    ["generate", "--config", "noli.yaml", "--apply", "--format", "json"],
    context.cwd,
    options.timeoutMs,
  );
  const report = (await runNoliArgv(
    binaryPath,
    ["validate", "--root", "knowledge", "--mode", "project", "--config", "noli.yaml", "--format", "json"],
    context.cwd,
    options.timeoutMs,
  )) as { valid?: boolean };
  if (report?.valid !== true) {
    throw new NoliError("bootstrap validation reported an invalid bundle", "VALIDATION_FAILED");
  }
  context.ui.notify(
    "Noli knowledge base initialized. Author real concepts in .noli/concepts.yaml " +
      "and re-run: noli generate --config noli.yaml --apply",
    "info",
  );
}

/** Finds a starter template next to the installed extension. */
function starterPath(name: string, explicit: string | undefined): string {
  if (explicit) {
    return explicit;
  }
  const directory =
    typeof import.meta.dirname === "string"
      ? import.meta.dirname
      : path.dirname(fileURLToPath(import.meta.url));
  const candidates = [
    path.join(directory, name),
    path.join(directory, "..", "shared", name),
  ];
  for (const candidate of candidates) {
    if (existsSync(candidate)) {
      return candidate;
    }
  }
  throw new NoliError(`starter template ${name} was not found next to the extension`, "INTERNAL_ERROR");
}

/**
 * Runs one bootstrap noli command with a bounded lifetime and returns the
 * envelope's data payload. Argument arrays only, no shell; distinct from the
 * frozen five-operation runner, which stays read-only.
 */
function runNoliArgv(
  binaryPath: string,
  args: string[],
  cwd: string,
  timeoutMs = 60_000,
): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const child = spawn(binaryPath, args, { cwd, shell: false, stdio: ["ignore", "pipe", "ignore"] });
    const chunks: Buffer[] = [];
    let settled = false;
    const timer = setTimeout(() => {
      settle(() => {
        child.kill("SIGKILL");
        child.stdout.destroy();
        reject(new NoliError(`noli ${args[0]} timed out after ${timeoutMs}ms`, "TIMEOUT"));
      });
    }, timeoutMs);
    const settle = (finish: () => void): void => {
      if (!settled) {
        settled = true;
        clearTimeout(timer);
        finish();
      }
    };
    child.stdout.on("data", (chunk: Buffer) => chunks.push(Buffer.from(chunk)));
    child.on("error", (error) => {
      settle(() => reject(new NoliError(`failed to launch noli: ${error.message}`, "INTERNAL_ERROR")));
    });
    child.on("close", (exitCode) => {
      settle(() => {
        let envelope: { ok?: boolean; data?: unknown; error?: { code?: string; message?: string } };
        try {
          envelope = JSON.parse(Buffer.concat(chunks).toString("utf8")) as typeof envelope;
        } catch {
          reject(new NoliError(`noli ${args[0]} returned invalid JSON (exit ${exitCode})`, "INVALID_JSON", exitCode));
          return;
        }
        if (envelope.ok !== true) {
          reject(new NoliError(
            envelope.error?.message ?? `noli ${args[0]} failed with exit code ${exitCode}`,
            envelope.error?.code ?? "INTERNAL_ERROR",
            exitCode,
          ));
          return;
        }
        resolve(envelope.data);
      });
    });
  });
}

/** Pi's required default extension factory. */
export default function noliExtension(pi: PiExtensionAPI): void {
  const tools = toolsWithOptions((context) => ({
    binaryPath: configuredBinary(),
    repositoryRoot: configuredRepository(context.cwd),
  }));
  for (const tool of tools) {
    pi.registerTool(tool);
  }
  pi.on?.("session_start", (event, context) => {
    // Fire-and-forget: Pi awaits session_start handlers during startup, so
    // the handler returns immediately and the dialog runs detached.
    void runSessionStart(event, context);
  });
}
