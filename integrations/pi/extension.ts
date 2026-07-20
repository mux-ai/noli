// Noli Pi extension exposing five bounded, read-only OKF tools.
//
// The shapes below follow Pi 0.78's ExtensionAPI/ToolDefinition contract while
// remaining structural, so this adapter does not need to bundle Pi itself.
// Pi's loader supplies the ExtensionAPI and validates the JSON schemas.
import { accessSync, constants, existsSync, realpathSync } from "node:fs";
import path from "node:path";

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

/** The verified subset of Pi's ExtensionAPI used by this extension. */
export interface PiExtensionAPI {
  registerTool(tool: NoliToolDefinition): void;
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

/** Pi's required default extension factory. */
export default function noliExtension(pi: PiExtensionAPI): void {
  const tools = toolsWithOptions((context) => ({
    binaryPath: configuredBinary(),
    repositoryRoot: configuredRepository(context.cwd),
  }));
  for (const tool of tools) {
    pi.registerTool(tool);
  }
}
