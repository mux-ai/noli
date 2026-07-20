import assert from "node:assert/strict";
import { chmodSync, existsSync, mkdirSync, mkdtempSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { test } from "node:test";

import noliExtension, { createNoliTools, register, type NoliToolDefinition } from "./extension.ts";
import { NoliError, buildArguments, resolveKnowledgeRoot, runNoli } from "./runner.ts";

const SUCCESS_ENVELOPE = JSON.stringify({
  ok: true,
  command: "status",
  version: 1,
  data: { root: "knowledge", document_count: 3 },
});

const ERROR_ENVELOPE = JSON.stringify({
  ok: false,
  command: "get",
  version: 1,
  error: { code: "DOCUMENT_NOT_FOUND", message: "document \"x\" was not found" },
});

interface Fixture {
  repository: string;
  binary(script: string): string;
}

function fixture(): Fixture {
  const base = mkdtempSync(path.join(tmpdir(), "noli-pi-test-"));
  const repository = path.join(base, "repo");
  mkdirSync(path.join(repository, "knowledge"), { recursive: true });
  let counter = 0;
  return {
    repository,
    binary(script: string): string {
      counter += 1;
      const file = path.join(base, `fake-noli-${counter}.sh`);
      writeFileSync(file, "#!/bin/sh\n" + script + "\n");
      chmodSync(file, 0o755);
      return file;
    },
  };
}

test("valid subprocess JSON resolves to the data payload", async () => {
  const f = fixture();
  const data = (await runNoli(
    { operation: "status", root: "knowledge" },
    { binaryPath: f.binary(`printf '%s\\n' '${SUCCESS_ENVELOPE}'`), repositoryRoot: f.repository },
  )) as { document_count: number };
  assert.equal(data.document_count, 3);
});

test("UTF-8 split across stdout chunks is preserved", async () => {
  const f = fixture();
  const binary = f.binary(
    "printf '{\"ok\":true,\"command\":\"status\",\"version\":1,\"data\":{\"text\":\"'; " +
      "printf '\\360\\237'; sleep 0.05; printf '\\230\\200\"}}\\n'",
  );
  const data = (await runNoli(
    { operation: "status", root: "knowledge" },
    { binaryPath: binary, repositoryRoot: f.repository },
  )) as { text: string };
  assert.equal(data.text, "😀");
});

test("successful JSON with stderr violates the protocol", async () => {
  const f = fixture();
  await assert.rejects(
    runNoli(
      { operation: "status", root: "knowledge" },
      {
        binaryPath: f.binary(`printf '%s\\n' '${SUCCESS_ENVELOPE}'; echo diagnostic >&2`),
        repositoryRoot: f.repository,
      },
    ),
    (error: NoliError) => error.code === "INVALID_PROTOCOL",
  );
});

test("non-zero exit with an error envelope surfaces the stable code", async () => {
  const f = fixture();
  await assert.rejects(
    runNoli(
      { operation: "get", root: "knowledge", id: "x" },
      { binaryPath: f.binary(`printf '%s\\n' '${ERROR_ENVELOPE}'; exit 3`), repositoryRoot: f.repository },
    ),
    (error: NoliError) => error.code === "DOCUMENT_NOT_FOUND" && error.exitCode === 3,
  );
});

test("invalid JSON is rejected with INVALID_JSON", async () => {
  const f = fixture();
  await assert.rejects(
    runNoli(
      { operation: "status", root: "knowledge" },
      { binaryPath: f.binary("echo not-json-at-all"), repositoryRoot: f.repository },
    ),
    (error: NoliError) => error.code === "INVALID_JSON",
  );
});

test("oversized stdout kills the process and rejects", async () => {
  const f = fixture();
  await assert.rejects(
    runNoli(
      { operation: "status", root: "knowledge" },
      {
        binaryPath: f.binary("head -c 300000 /dev/zero | tr '\\0' 'x'"),
        repositoryRoot: f.repository,
        maxOutputBytes: 64 * 1024,
      },
    ),
    (error: NoliError) => error.code === "OUTPUT_LIMIT",
  );
});

test("oversized stderr kills the process and rejects", async () => {
  const f = fixture();
  await assert.rejects(
    runNoli(
      { operation: "status", root: "knowledge" },
      {
        binaryPath: f.binary("head -c 300000 /dev/zero | tr '\\0' 'x' >&2"),
        repositoryRoot: f.repository,
        maxOutputBytes: 64 * 1024,
      },
    ),
    (error: NoliError) => error.code === "OUTPUT_LIMIT",
  );
});

test("timeout kills the process and rejects", async () => {
  const f = fixture();
  const started = Date.now();
  await assert.rejects(
    runNoli(
      { operation: "status", root: "knowledge" },
      { binaryPath: f.binary("sleep 30"), repositoryRoot: f.repository, timeoutMs: 200 },
    ),
    (error: NoliError) => error.code === "TIMEOUT",
  );
  assert.ok(Date.now() - started < 5000, "process was not killed promptly");
});

test("abort signal kills the process and rejects promptly", async () => {
  const f = fixture();
  const controller = new AbortController();
  const started = Date.now();
  const running = runNoli(
    { operation: "status", root: "knowledge" },
    {
      binaryPath: f.binary("sleep 30"),
      repositoryRoot: f.repository,
      signal: controller.signal,
    },
  );
  setTimeout(() => controller.abort(), 50);
  await assert.rejects(running, (error: NoliError) => error.code === "ABORTED");
  assert.ok(Date.now() - started < 5000, "aborted process was not killed promptly");
});

test("operations outside the allowlist are rejected without spawning", async () => {
  const f = fixture();
  for (const operation of ["generate", "prepare-agent-context", "validate", "rm"]) {
    await assert.rejects(
      runNoli(
        { operation: operation as never, root: "knowledge" },
        { binaryPath: "/nonexistent", repositoryRoot: f.repository },
      ),
      (error: NoliError) => error.code === "INVALID_ARGUMENT",
    );
  }
});

test("knowledge roots escaping the repository are rejected", () => {
  const f = fixture();
  assert.throws(
    () => resolveKnowledgeRoot(f.repository, "../outside"),
    (error: NoliError) => error.code === "KNOWLEDGE_NOT_FOUND" || error.code === "UNSAFE_PATH",
  );
  assert.throws(
    () => resolveKnowledgeRoot(f.repository, "know\0ledge"),
    (error: NoliError) => error.code === "INVALID_ARGUMENT",
  );
});

test("the .noli/disabled sentinel turns every operation off", async () => {
  const f = fixture();
  mkdirSync(path.join(f.repository, ".noli"), { recursive: true });
  writeFileSync(path.join(f.repository, ".noli", "disabled"), "developer opted out\n");
  await assert.rejects(
    runNoli(
      { operation: "status", root: "knowledge" },
      { binaryPath: f.binary(`printf '%s\\n' '${SUCCESS_ENVELOPE}'`), repositoryRoot: f.repository },
    ),
    (error: NoliError) => error.code === "NOLI_DISABLED",
  );
});

test("the legacy .okf/disabled sentinel remains a fallback", async () => {
  const f = fixture();
  mkdirSync(path.join(f.repository, ".okf"), { recursive: true });
  writeFileSync(path.join(f.repository, ".okf", "disabled"), "developer opted out\n");
  await assert.rejects(
    runNoli(
      { operation: "status", root: "knowledge" },
      { binaryPath: f.binary(`printf '%s\\n' '${SUCCESS_ENVELOPE}'`), repositoryRoot: f.repository },
    ),
    (error: NoliError) => error.code === "NOLI_DISABLED",
  );
});

test("the Noli namespace overrides a legacy disabled sentinel", async () => {
  const f = fixture();
  mkdirSync(path.join(f.repository, ".noli"), { recursive: true });
  mkdirSync(path.join(f.repository, ".okf"), { recursive: true });
  writeFileSync(path.join(f.repository, ".okf", "disabled"), "legacy opt-out\n");
  const data = (await runNoli(
    { operation: "status", root: "knowledge" },
    { binaryPath: f.binary(`printf '%s\\n' '${SUCCESS_ENVELOPE}'`), repositoryRoot: f.repository },
  )) as { document_count: number };
  assert.equal(data.document_count, 3);
});

test("symlink escapes are rejected after resolution", () => {
  const f = fixture();
  const outside = mkdtempSync(path.join(tmpdir(), "noli-pi-outside-"));
  symlinkSync(outside, path.join(f.repository, "linked"));
  assert.throws(
    () => resolveKnowledgeRoot(f.repository, "linked"),
    (error: NoliError) => error.code === "UNSAFE_PATH",
  );
});

test("argument building validates bounds and directions", () => {
  const f = fixture();
  const root = resolveKnowledgeRoot(f.repository, "knowledge");
  const args = buildArguments(
    {
      operation: "retrieve",
      root: "knowledge",
      query: "complete a task",
      maxDocuments: 8,
      direction: "both",
      types: ["Business Rule", "Domain Entity"],
    },
    root,
  );
  assert.deepEqual(args, [
    "retrieve",
    "--root",
    root,
    "--format",
    "json",
    "--query",
    "complete a task",
    "--max-documents",
    "8",
    "--direction",
    "both",
    "--types",
    "Business Rule,Domain Entity",
  ]);
  assert.throws(
    () => buildArguments({ operation: "retrieve", root: "knowledge", query: "x", maxHops: -1 }, root),
    (error: NoliError) => error.code === "INVALID_ARGUMENT",
  );
  assert.throws(
    () => buildArguments({ operation: "graph", root: "knowledge", id: "a", direction: "sideways" }, root),
    (error: NoliError) => error.code === "INVALID_ARGUMENT",
  );
  assert.throws(
    () => buildArguments({ operation: "search", root: "knowledge" }, root),
    (error: NoliError) => error.code === "INVALID_ARGUMENT",
  );
});

test("extension exposes exactly the five allowlisted tools", () => {
  const f = fixture();
  const tools = createNoliTools({ binaryPath: "/usr/local/bin/noli", repositoryRoot: f.repository });
  assert.deepEqual(
    tools.map((tool) => tool.name),
    ["noli_status", "noli_search", "noli_retrieve", "noli_get", "noli_graph"],
  );
  assert.deepEqual(
    tools.map((tool) => tool.label),
    ["Noli Status", "Noli Search", "Noli Retrieve", "Noli Get", "Noli Graph"],
  );
  const registered: string[] = [];
  register(
    { registerTool: (tool: NoliToolDefinition) => registered.push(tool.name) },
    { binaryPath: "/usr/local/bin/noli", repositoryRoot: f.repository },
  );
  assert.equal(registered.length, 5);

  const defaultRegistered: string[] = [];
  noliExtension({ registerTool: (tool: NoliToolDefinition) => defaultRegistered.push(tool.name) });
  assert.deepEqual(defaultRegistered, registered);
  assert.ok(tools.every((tool) => tool.label && tool.executionMode === "parallel"));
});

test("tool execution runs against the real noli binary when available", async (t) => {
  const repositoryRoot = path.resolve(import.meta.dirname, "..", "..");
  const binaryPath = path.join(repositoryRoot, "bin", "noli");
  if (!existsSync(binaryPath)) {
    t.skip("bin/noli not built");
    return;
  }
  const tools = createNoliTools({ binaryPath, repositoryRoot });
  const status = tools.find((tool) => tool.name === "noli_status");
  assert.ok(status);
	const statusResult = await status.execute(
		"status-call",
		{ root: "examples/todo-app/knowledge" },
		undefined,
		undefined,
		{ cwd: repositoryRoot },
	);
	const data = statusResult.details.data as {
    document_count: number;
  };
  assert.equal(data.document_count, 18);
  const retrieve = tools.find((tool) => tool.name === "noli_retrieve");
  assert.ok(retrieve);
	const retrieveResult = await retrieve.execute(
		"retrieve-call",
		{
			root: "examples/todo-app/knowledge",
			query: "Implement the CompleteTodo use case",
			types: ["Business Rule", "Domain Entity", "Application Component", "Architecture Decision"],
			maxDocuments: 8,
			maxCharacters: 14000,
			direction: "both",
		},
		undefined,
		undefined,
		{ cwd: repositoryRoot },
	);
	const retrieved = retrieveResult.details.data as { sources: Array<{ id: string; seed: boolean }> };
  assert.equal(retrieved.sources[0].id, "rules/complete-task");
  assert.equal(retrieved.sources[0].seed, true);
});
