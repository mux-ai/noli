package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const prepareQueries = `version: 1
queries:
  - name: complete-todo
    query: Complete Task
    max_characters: 14000
`

func TestPrepareAgentContextCommand(t *testing.T) {
	root := knowledgeFixture(t)
	dir := t.TempDir()
	queriesPath := filepath.Join(dir, "noli-agent-queries.yaml")
	if err := os.WriteFile(queriesPath, []byte(prepareQueries), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "prepared")

	args := []string{
		"prepare-agent-context", "--root", root, "--config", queriesPath,
		"--output", output, "--timestamp", "2026-07-20T00:00:00Z",
	}
	exit, envelope, _, stderr := runCLI(t, args...)
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	data := envelope["data"].(map[string]any)
	if data["generated_at"] != "2026-07-20T00:00:00Z" || data["manifest"] != "manifest.json" {
		t.Fatalf("data = %#v", data)
	}
	queries := data["queries"].([]any)
	if len(queries) != 1 {
		t.Fatalf("queries = %#v", queries)
	}
	first := queries[0].(map[string]any)
	if first["file"] != "complete-todo.md" {
		t.Fatalf("first = %#v", first)
	}
	sources := first["sources"].([]any)
	if len(sources) == 0 || sources[0] != "rules/complete-task" {
		t.Fatalf("sources = %#v", sources)
	}
	if _, err := os.Stat(filepath.Join(output, "complete-todo.md")); err != nil {
		t.Fatalf("context file missing: %v", err)
	}
	var manifest map[string]any
	manifestData, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("manifest is not JSON: %v", err)
	}

	// Re-running with the same timestamp reproduces identical output.
	_, envelope2, first1, _ := runCLI(t, args...)
	_, _, first2, _ := runCLI(t, args...)
	if first1 != first2 {
		t.Fatal("prepare output not deterministic")
	}
	_ = envelope2
}

func TestPrepareAgentContextErrors(t *testing.T) {
	root := knowledgeFixture(t)
	dir := t.TempDir()
	queriesPath := filepath.Join(dir, "noli-agent-queries.yaml")
	if err := os.WriteFile(queriesPath, []byte(prepareQueries), 0o644); err != nil {
		t.Fatal(err)
	}

	exit, envelope, _, _ := runCLI(t,
		"prepare-agent-context", "--root", root, "--config", queriesPath)
	if exit != 2 || errorCode(t, envelope) != "INVALID_ARGUMENT" {
		t.Fatalf("missing --output: exit %d (%#v)", exit, envelope)
	}

	exit, envelope, _, _ = runCLI(t,
		"prepare-agent-context", "--root", root, "--config", queriesPath,
		"--output", filepath.Join(dir, "out"), "--timestamp", "not-a-time")
	if exit != 2 || errorCode(t, envelope) != "INVALID_ARGUMENT" {
		t.Fatalf("bad timestamp: exit %d (%#v)", exit, envelope)
	}

	exit, envelope, _, _ = runCLI(t,
		"prepare-agent-context", "--root", root,
		"--config", filepath.Join(dir, "missing.yaml"), "--output", filepath.Join(dir, "out"))
	if exit != 3 || errorCode(t, envelope) != "KNOWLEDGE_NOT_FOUND" {
		t.Fatalf("missing config: exit %d (%#v)", exit, envelope)
	}

	badQueries := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badQueries, []byte("version: 1\nqueries:\n  - name: ../escape\n    query: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exit, envelope, _, _ = runCLI(t,
		"prepare-agent-context", "--root", root, "--config", badQueries,
		"--output", filepath.Join(dir, "out"))
	if exit != 3 || errorCode(t, envelope) != "PARSE_ERROR" {
		t.Fatalf("bad query name: exit %d (%#v)", exit, envelope)
	}

	exit, envelope, _, _ = runCLI(t,
		"prepare-agent-context", "--root", root, "--config", queriesPath,
		"--output", filepath.Join(root, "inside"))
	if exit != 6 || errorCode(t, envelope) != "UNSAFE_PATH" {
		t.Fatalf("output inside root: exit %d (%#v)", exit, envelope)
	}

	foreign := t.TempDir()
	if err := os.WriteFile(filepath.Join(foreign, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	exit, envelope, _, _ = runCLI(t,
		"prepare-agent-context", "--root", root, "--config", queriesPath,
		"--output", foreign)
	if exit != 6 || errorCode(t, envelope) != "UNSAFE_PATH" {
		t.Fatalf("foreign output: exit %d (%#v)", exit, envelope)
	}

	tinyQueries := filepath.Join(dir, "tiny.yaml")
	if err := os.WriteFile(tinyQueries,
		[]byte("version: 1\nqueries:\n  - name: tiny\n    query: Complete Task\n    max_characters: 10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exit, envelope, _, _ = runCLI(t,
		"prepare-agent-context", "--root", root, "--config", tinyQueries,
		"--output", filepath.Join(dir, "out2"))
	if exit != 2 || errorCode(t, envelope) != "CONTEXT_LIMIT_TOO_SMALL" {
		t.Fatalf("tiny budget: exit %d (%#v)", exit, envelope)
	}
}
