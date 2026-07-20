package cli

import (
	"os"
	"path/filepath"
	"testing"
)

const projectConfig = `version: 1
project:
  name: Todo App
knowledge:
  root: knowledge
concept_types:
  - type: Domain Entity
    directory: concepts
  - type: Business Rule
    directory: rules
`

// projectFixture builds a project directory holding noli.yaml plus the
// knowledge bundle from knowledgeFixture.
func projectFixture(t *testing.T, config string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "noli.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	knowledge := filepath.Join(dir, "knowledge")
	writeFixtureFile(t, filepath.Join(knowledge, "index.md"),
		"---\ntype: Navigation\ntitle: Home\n---\n\n- [Concepts](concepts/)\n")
	writeFixtureFile(t, filepath.Join(knowledge, "log.md"),
		"---\ntype: Bundle Log\ntitle: Log\n---\n\nentry\n")
	writeFixtureFile(t, filepath.Join(knowledge, "concepts", "index.md"),
		"---\ntype: Navigation\ntitle: Concepts\n---\n\n- [Todo Item](todo-item.md)\n")
	writeFixtureFile(t, filepath.Join(knowledge, "concepts", "todo-item.md"),
		"---\ntype: Domain Entity\ntitle: Todo Item\n---\n\n## Definition\n\nbody\n")
	writeFixtureFile(t, filepath.Join(knowledge, "rules", "index.md"),
		"---\ntype: Navigation\ntitle: Rules\n---\n\n- [Complete Task](complete-task.md)\n")
	writeFixtureFile(t, filepath.Join(knowledge, "rules", "complete-task.md"),
		"---\ntype: Business Rule\ntitle: Complete Task\n---\n\n## Statement\n\ntext\n")
	return dir
}

func TestValidateProjectModeSucceeds(t *testing.T) {
	dir := projectFixture(t, projectConfig)
	exit, envelope, _, stderr := runCLI(t,
		"validate", "--mode", "project", "--config", filepath.Join(dir, "noli.yaml"))
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	data := envelope["data"].(map[string]any)
	if data["mode"] != "project" || data["valid"] != true {
		t.Fatalf("data = %#v", data)
	}
}

func TestValidateProjectModeInvalidBundleExitFour(t *testing.T) {
	dir := projectFixture(t, projectConfig)
	writeFixtureFile(t, filepath.Join(dir, "knowledge", "concepts", "mystery.md"),
		"---\ntype: Mystery\ntitle: M\n---\n\nbody\n")
	exit, envelope, _, _ := runCLI(t,
		"validate", "--mode", "project", "--config", filepath.Join(dir, "noli.yaml"))
	if exit != 4 || envelope["ok"] != true {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	data := envelope["data"].(map[string]any)
	found := false
	for _, problem := range data["errors"].([]any) {
		if problem.(map[string]any)["code"] == "UNKNOWN_TYPE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("UNKNOWN_TYPE missing: %#v", data)
	}
}

func TestValidateProjectModeConfigErrors(t *testing.T) {
	dir := projectFixture(t, projectConfig)

	exit, envelope, _, _ := runCLI(t, "validate", "--mode", "project")
	if exit != 2 || errorCode(t, envelope) != "INVALID_ARGUMENT" {
		t.Fatalf("missing --config: exit %d (%#v)", exit, envelope)
	}

	exit, envelope, _, _ = runCLI(t,
		"validate", "--mode", "project", "--config", filepath.Join(dir, "missing.yaml"))
	if exit != 3 || errorCode(t, envelope) != "KNOWLEDGE_NOT_FOUND" {
		t.Fatalf("missing config: exit %d (%#v)", exit, envelope)
	}

	badPathDir := projectFixture(t, projectConfig)
	unsafe := "version: 1\nproject:\n  name: X\nknowledge:\n  root: /etc/knowledge\nconcept_types:\n  - type: T\n    directory: concepts\n"
	if err := os.WriteFile(filepath.Join(badPathDir, "noli.yaml"), []byte(unsafe), 0o644); err != nil {
		t.Fatal(err)
	}
	exit, envelope, _, _ = runCLI(t,
		"validate", "--mode", "project", "--config", filepath.Join(badPathDir, "noli.yaml"))
	if exit != 6 || errorCode(t, envelope) != "UNSAFE_PATH" {
		t.Fatalf("unsafe config: exit %d (%#v)", exit, envelope)
	}

	brokenDir := projectFixture(t, projectConfig)
	if err := os.WriteFile(filepath.Join(brokenDir, "noli.yaml"),
		[]byte(projectConfig+"unexpected_field: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exit, envelope, _, _ = runCLI(t,
		"validate", "--mode", "project", "--config", filepath.Join(brokenDir, "noli.yaml"))
	if exit != 3 || errorCode(t, envelope) != "PARSE_ERROR" {
		t.Fatalf("strict parse: exit %d (%#v)", exit, envelope)
	}
}

func TestValidateProjectModeRootOverride(t *testing.T) {
	dir := projectFixture(t, projectConfig)
	exit, envelope, _, _ := runCLI(t,
		"validate", "--mode", "project",
		"--config", filepath.Join(dir, "noli.yaml"),
		"--root", filepath.Join(dir, "knowledge"))
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
}
