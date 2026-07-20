import assert from "node:assert/strict";
import { accessSync, constants, existsSync, realpathSync } from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

function findOnPath(name) {
  for (const entry of (process.env.PATH ?? "").split(path.delimiter)) {
    if (!entry) continue;
    const candidate = path.resolve(entry, process.platform === "win32" ? `${name}.exe` : name);
    try {
      accessSync(candidate, constants.X_OK);
      return realpathSync(candidate);
    } catch {
      // Continue through PATH.
    }
  }
  throw new Error("Pi is required for the loader compatibility test but was not found on PATH");
}

const piBinary = findOnPath("pi");
const piPackage = path.dirname(path.dirname(piBinary));
const loaderPath = path.join(piPackage, "dist", "core", "extensions", "loader.js");
if (!existsSync(loaderPath)) {
  throw new Error(`installed Pi does not expose the expected extension loader: ${loaderPath}`);
}

const integrationDir = import.meta.dirname;
const repositoryRoot = path.resolve(integrationDir, "..", "..");
const extensionPath = path.join(integrationDir, "extension.ts");
const { loadExtensions } = await import(pathToFileURL(loaderPath).href);
const loaded = await loadExtensions([extensionPath], repositoryRoot);

assert.deepEqual(loaded.errors, []);
assert.equal(loaded.extensions.length, 1);
assert.deepEqual([...loaded.extensions[0].tools.keys()], [
  "noli_status",
  "noli_search",
  "noli_retrieve",
  "noli_get",
  "noli_graph",
]);

const binaryPath = path.join(repositoryRoot, "bin", "noli");
if (existsSync(binaryPath)) {
  process.env.NOLI_BINARY_PATH = binaryPath;
  const status = loaded.extensions[0].tools.get("noli_status").definition;
  const result = await status.execute(
    "loader-smoke-test",
    { root: "examples/todo-app/knowledge" },
    undefined,
    undefined,
    { cwd: repositoryRoot },
  );
  const data = result.details.data;
  assert.equal(data.document_count, 18);

  delete process.env.NOLI_BINARY_PATH;
  process.env.OKF_BINARY_PATH = binaryPath;
  const legacyResult = await status.execute(
    "loader-legacy-environment-smoke-test",
    { root: "examples/todo-app/knowledge" },
    undefined,
    undefined,
    { cwd: repositoryRoot },
  );
  assert.equal(legacyResult.details.data.document_count, 18);
  delete process.env.OKF_BINARY_PATH;
}

console.log("Noli Pi loader compatibility: PASS (5 tools registered)");
