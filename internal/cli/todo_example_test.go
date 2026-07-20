package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

const todoExample = "../../examples/todo-app"

// TestTodoExampleCompleteTodoRetrieval is the end-to-end acceptance flow
// from PLANS.md section 10: the CompleteTodo retrieval must select exactly
// the expected sources in deterministic bounded order.
func TestTodoExampleCompleteTodoRetrieval(t *testing.T) {
	root := filepath.Join(todoExample, "knowledge")
	exit, envelope, output, stderr := runCLI(t,
		"retrieve", "--root", root,
		"--query", "Implement the CompleteTodo use case",
		"--types", "Business Rule,Domain Entity,Application Component,Architecture Decision",
		"--search-limit", "5", "--max-hops", "1",
		"--max-documents", "8", "--max-characters", "14000",
		"--direction", "both", "--format", "json")
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	data := envelope["data"].(map[string]any)
	sources := data["sources"].([]any)
	got := make([]string, len(sources))
	for i, source := range sources {
		got[i] = source.(map[string]any)["id"].(string)
	}
	want := []string{
		"rules/complete-task",
		"concepts/todo-item",
		"concepts/task-status",
		"components/todo-service",
		"decisions/domain-validation-layer",
		"components/todo-repository",
	}
	if len(got) != len(want) {
		t.Fatalf("sources = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sources = %v, want %v", got, want)
		}
	}

	first := sources[0].(map[string]any)
	if first["seed"] != true {
		t.Fatalf("business rule is not a seed: %#v", first)
	}
	for _, source := range sources[1:] {
		if source.(map[string]any)["seed"] == true {
			t.Fatalf("unexpected extra seed: %#v", source)
		}
	}
	for _, source := range sources {
		id := source.(map[string]any)["id"].(string)
		if id == "index" || id == "log" || strings.HasSuffix(id, "/index") {
			t.Fatalf("navigation or log document selected: %s", id)
		}
	}
	context := data["context"].(string)
	if utf8.RuneCountInString(context) > 14000 {
		t.Fatalf("context exceeds 14000 characters: %d", utf8.RuneCountInString(context))
	}
	statistics := data["statistics"].(map[string]any)
	if statistics["truncated"] != false {
		t.Fatalf("statistics = %#v", statistics)
	}

	// Determinism: byte-identical across runs.
	_, _, second, _ := runCLI(t,
		"retrieve", "--root", root,
		"--query", "Implement the CompleteTodo use case",
		"--types", "Business Rule,Domain Entity,Application Component,Architecture Decision",
		"--search-limit", "5", "--max-hops", "1",
		"--max-documents", "8", "--max-characters", "14000",
		"--direction", "both", "--format", "json")
	if output != second {
		t.Fatal("Todo retrieval output differs across runs")
	}
}

// TestTodoExampleValidatesInBothModes proves the shipped example passes
// standard and project validation without invented confidence or citation
// values.
func TestTodoExampleValidatesInBothModes(t *testing.T) {
	root := filepath.Join(todoExample, "knowledge")
	exit, envelope, _, _ := runCLI(t, "validate", "--root", root, "--mode", "standard")
	if exit != 0 {
		t.Fatalf("standard: exit %d (%#v)", exit, envelope)
	}
	exit, envelope, _, _ = runCLI(t,
		"validate", "--mode", "project",
		"--config", filepath.Join(todoExample, "noli.yaml"),
		"--root", root)
	if exit != 0 {
		t.Fatalf("project: exit %d (%#v)", exit, envelope)
	}
	data := envelope["data"].(map[string]any)
	if data["valid"] != true || len(data["errors"].([]any)) != 0 {
		t.Fatalf("data = %#v", data)
	}
}

// TestTodoExamplePreparedQueriesParse proves the shipped agent queries file
// parses and prepares against the example bundle.
func TestTodoExamplePreparedQueries(t *testing.T) {
	root := filepath.Join(todoExample, "knowledge")
	output := filepath.Join(t.TempDir(), "prepared")
	exit, envelope, _, _ := runCLI(t,
		"prepare-agent-context", "--root", root,
		"--config", filepath.Join(todoExample, "noli-agent-queries.yaml"),
		"--output", output, "--timestamp", "2026-07-20T00:00:00Z")
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	queries := envelope["data"].(map[string]any)["queries"].([]any)
	if len(queries) != 2 {
		t.Fatalf("queries = %#v", queries)
	}
	first := queries[0].(map[string]any)
	if first["name"] != "complete-todo" {
		t.Fatalf("first query = %#v", first)
	}
	sources := first["sources"].([]any)
	if len(sources) != 6 || sources[0] != "rules/complete-task" {
		t.Fatalf("sources = %#v", sources)
	}
}
