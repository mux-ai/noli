package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func knowledgeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "index.md"),
		"---\ntype: Navigation\ntitle: Home\n---\n\n- [Concepts](concepts/)\n")
	writeFixtureFile(t, filepath.Join(root, "log.md"),
		"---\ntype: Bundle Log\ntitle: Log\n---\n\n## 2026-07-20\n")
	writeFixtureFile(t, filepath.Join(root, "concepts", "index.md"),
		"---\ntype: Navigation\ntitle: Concepts\n---\n\n- [Todo Item](todo-item.md)\n")
	writeFixtureFile(t, filepath.Join(root, "concepts", "todo-item.md"),
		"---\ntype: Domain Entity\ntitle: Todo Item\ntags: [domain]\nseverity: must\n---\n\n## Definition\n\n- Uses: [Status](task-status.md)\n")
	writeFixtureFile(t, filepath.Join(root, "concepts", "task-status.md"),
		"---\ntype: Domain Entity\ntitle: Status Lifecycle\n---\n\n## States\n")
	writeFixtureFile(t, filepath.Join(root, "rules", "index.md"),
		"---\ntype: Navigation\ntitle: Rules\n---\n\n- [Complete Task](complete-task.md)\n")
	writeFixtureFile(t, filepath.Join(root, "rules", "complete-task.md"),
		"---\ntype: Business Rule\ntitle: Complete Task\n---\n\n## Statement\n\n- Applies to: [Todo Item](../concepts/todo-item.md)\n")
	return root
}

// runCLI executes one invocation and decodes the JSON envelope.
func runCLI(t *testing.T, args ...string) (int, map[string]any, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exit := New(&stdout, &stderr).Run(args)
	output := stdout.String()
	if output == "" {
		t.Fatalf("no stdout for args %v", args)
	}
	if strings.Count(output, "\n") != 1 || !strings.HasSuffix(output, "\n") {
		t.Fatalf("stdout must be one JSON line: %q", output)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("stdout is not JSON: %v (%q)", err, output)
	}
	return exit, envelope, output, stderr.String()
}

func errorCode(t *testing.T, envelope map[string]any) string {
	t.Helper()
	detail, ok := envelope["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error detail: %#v", envelope)
	}
	code, _ := detail["code"].(string)
	return code
}

func TestSuccessCommandsEmitJSONOnlyToStdout(t *testing.T) {
	root := knowledgeFixture(t)
	cases := [][]string{
		{"status", "--root", root, "--format", "json"},
		{"list", "--root", root},
		{"search", "--root", root, "--query", "Complete Task", "--limit", "5"},
		{"retrieve", "--root", root, "--query", "Complete Task", "--max-hops", "1"},
		{"get", "--root", root, "--id", "rules/complete-task"},
		{"graph", "--root", root, "--id", "rules/complete-task", "--direction", "both"},
		{"validate", "--root", root, "--mode", "standard"},
	}
	for _, args := range cases {
		exit, envelope, _, stderr := runCLI(t, args...)
		if exit != 0 {
			t.Fatalf("%v exit = %d (%#v)", args, exit, envelope)
		}
		if stderr != "" {
			t.Fatalf("%v wrote to stderr: %q", args, stderr)
		}
		if envelope["ok"] != true || envelope["command"] != args[0] || envelope["version"] != float64(1) {
			t.Fatalf("%v envelope = %#v", args, envelope)
		}
		if _, hasData := envelope["data"]; !hasData {
			t.Fatalf("%v missing data: %#v", args, envelope)
		}
	}
}

func TestDeterministicOutputAcrossRuns(t *testing.T) {
	root := knowledgeFixture(t)
	commands := [][]string{
		{"status", "--root", root},
		{"list", "--root", root},
		{"search", "--root", root, "--query", "Complete Task"},
		{"retrieve", "--root", root, "--query", "Complete Task"},
		{"graph", "--root", root, "--id", "rules/complete-task"},
		{"validate", "--root", root},
	}
	for _, args := range commands {
		_, _, first, _ := runCLI(t, args...)
		_, _, second, _ := runCLI(t, args...)
		if first != second {
			t.Fatalf("%v output differs across runs:\n%s\n%s", args, first, second)
		}
	}
}

func TestStatusContent(t *testing.T) {
	root := knowledgeFixture(t)
	exit, envelope, _, _ := runCLI(t, "status", "--root", root)
	if exit != 0 {
		t.Fatalf("exit = %d", exit)
	}
	data := envelope["data"].(map[string]any)
	if data["document_count"] != float64(7) {
		t.Fatalf("document_count = %v", data["document_count"])
	}
	if !strings.HasPrefix(data["bundle_id"].(string), "sha256:") {
		t.Fatalf("bundle_id = %v", data["bundle_id"])
	}
	types := data["types"].([]any)
	if len(types) != 2 { // Business Rule, Domain Entity; navigation/log excluded
		t.Fatalf("types = %#v", types)
	}
}

func TestRetrieveContentAndSourceTraceability(t *testing.T) {
	root := knowledgeFixture(t)
	exit, envelope, _, _ := runCLI(t, "retrieve", "--root", root, "--query", "Complete Task")
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	data := envelope["data"].(map[string]any)
	sources := data["sources"].([]any)
	if len(sources) == 0 {
		t.Fatal("no sources returned")
	}
	first := sources[0].(map[string]any)
	if first["id"] != "rules/complete-task" || first["seed"] != true || first["score"] != float64(11) {
		t.Fatalf("first source = %#v", first)
	}
	context := data["context"].(string)
	if !strings.Contains(context, "## Source: rules/complete-task (Business Rule, seed, score 11)") {
		t.Fatalf("context = %q", context)
	}
	for _, source := range sources {
		id := source.(map[string]any)["id"].(string)
		if id == "index" || strings.HasSuffix(id, "/index") || id == "log" {
			t.Fatalf("navigation or log selected: %v", id)
		}
	}
}

func TestInvalidArgumentsExitTwo(t *testing.T) {
	root := knowledgeFixture(t)
	cases := []struct {
		args []string
		code string
	}{
		{[]string{}, "INVALID_ARGUMENT"},
		{[]string{"unknown-command"}, "INVALID_ARGUMENT"},
		{[]string{"search", "--root", root, "--query", "x", "--limit", "-1"}, "INVALID_ARGUMENT"},
		{[]string{"search", "--root", root}, "INVALID_ARGUMENT"},
		{[]string{"retrieve", "--root", root, "--query", "x", "--direction", "sideways"}, "INVALID_ARGUMENT"},
		{[]string{"status", "--root", root, "--format", "yaml"}, "INVALID_ARGUMENT"},
		{[]string{"status", "--root", root, "extra-positional"}, "INVALID_ARGUMENT"},
		{[]string{"validate", "--root", root, "--mode", "project"}, "INVALID_ARGUMENT"},
		{[]string{"status", "--unknown-flag"}, "INVALID_ARGUMENT"},
		{[]string{"retrieve", "--root", root, "--query", "Complete Task", "--max-characters", "10"}, "CONTEXT_LIMIT_TOO_SMALL"},
	}
	for _, testCase := range cases {
		exit, envelope, _, _ := runCLI(t, testCase.args...)
		if exit != 2 {
			t.Fatalf("%v exit = %d (%#v)", testCase.args, exit, envelope)
		}
		if envelope["ok"] != false || errorCode(t, envelope) != testCase.code {
			t.Fatalf("%v envelope = %#v", testCase.args, envelope)
		}
	}
}

func TestLoadFailuresExitThree(t *testing.T) {
	root := knowledgeFixture(t)
	missingRoot := filepath.Join(t.TempDir(), "missing")

	exit, envelope, _, _ := runCLI(t, "status", "--root", missingRoot)
	if exit != 3 || errorCode(t, envelope) != "KNOWLEDGE_NOT_FOUND" {
		t.Fatalf("missing root: exit %d %#v", exit, envelope)
	}

	exit, envelope, _, _ = runCLI(t, "get", "--root", root, "--id", "rules/missing")
	if exit != 3 || errorCode(t, envelope) != "DOCUMENT_NOT_FOUND" {
		t.Fatalf("missing document: exit %d %#v", exit, envelope)
	}

	exit, envelope, _, _ = runCLI(t, "graph", "--root", root, "--id", "rules/missing")
	if exit != 3 || errorCode(t, envelope) != "DOCUMENT_NOT_FOUND" {
		t.Fatalf("missing graph document: exit %d %#v", exit, envelope)
	}

	brokenRoot := t.TempDir()
	writeFixtureFile(t, filepath.Join(brokenRoot, "bad.md"), "no frontmatter\n")
	exit, envelope, _, _ = runCLI(t, "list", "--root", brokenRoot)
	if exit != 3 || errorCode(t, envelope) != "PARSE_ERROR" {
		t.Fatalf("broken bundle: exit %d %#v", exit, envelope)
	}

	frontmatterRoot := t.TempDir()
	writeFixtureFile(t, filepath.Join(frontmatterRoot, "bad.md"), "---\n- a\n- b\n---\n\nbody\n")
	exit, envelope, _, _ = runCLI(t, "list", "--root", frontmatterRoot)
	if exit != 3 || errorCode(t, envelope) != "INVALID_FRONTMATTER" {
		t.Fatalf("invalid frontmatter: exit %d %#v", exit, envelope)
	}
}

func TestValidateInvalidBundleExitFour(t *testing.T) {
	root := t.TempDir()
	// A missing type is the only standard-mode error (OKF v0.1 conformance
	// rule 2); broken links stay warnings per section 9.
	writeFixtureFile(t, filepath.Join(root, "rules", "untyped.md"),
		"---\ntitle: No Type\n---\n\n[missing](/concepts/missing.md)\n")
	exit, envelope, _, stderr := runCLI(t, "validate", "--root", root)
	if exit != 4 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	if envelope["ok"] != true {
		t.Fatalf("validate must stay a success envelope: %#v", envelope)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	data := envelope["data"].(map[string]any)
	if data["valid"] != false || len(data["errors"].([]any)) == 0 {
		t.Fatalf("data = %#v", data)
	}
}

func TestUnsafeRootExitSix(t *testing.T) {
	exit, envelope, _, _ := runCLI(t, "status", "--root", "bad\x00root")
	if exit != 6 || errorCode(t, envelope) != "UNSAFE_PATH" {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
}

func TestPanicBecomesInternalErrorExitSeven(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	app.commands["boom"] = func(args []string) int { panic("kaboom") }
	exit := app.Run([]string{"boom"})
	if exit != 7 {
		t.Fatalf("exit = %d", exit)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not JSON: %v (%q)", err, stdout.String())
	}
	if errorCode(t, envelope) != "INTERNAL_ERROR" {
		t.Fatalf("envelope = %#v", envelope)
	}
	if !strings.Contains(stderr.String(), "kaboom") {
		t.Fatalf("panic diagnostics missing from stderr: %q", stderr.String())
	}
}

func TestEmptyArraysAreNeverNull(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "index.md"), "---\ntype: Navigation\ntitle: Home\n---\n\ncontent\n")
	writeFixtureFile(t, filepath.Join(root, "log.md"), "---\ntype: Bundle Log\ntitle: Log\n---\n\ncontent\n")
	_, _, output, _ := runCLI(t, "list", "--root", root)
	if strings.Contains(output, "null") {
		t.Fatalf("output contains null: %q", output)
	}
	if !strings.Contains(output, "\"documents\":[]") {
		t.Fatalf("empty documents array missing: %q", output)
	}
	_, _, searchOutput, _ := runCLI(t, "search", "--root", root, "--query", "nothing-matches-zzz")
	if !strings.Contains(searchOutput, "\"results\":[]") || strings.Contains(searchOutput, "null") {
		t.Fatalf("empty results array missing: %q", searchOutput)
	}
}
