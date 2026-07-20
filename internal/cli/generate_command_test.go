package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const generateProjectConfig = `version: 1
project:
  name: Todo App
knowledge:
  root: knowledge
concept_types:
  - type: Domain Entity
    directory: concepts
    required_sections: [Definition]
relationships:
  - predicate: applies-to
generation:
  concept_files: [.noli/concepts.yaml]
`

const generateProjectConcepts = `concepts:
  - type: Domain Entity
    title: Todo Item
    description: The core task entity.
    sections:
      - heading: Definition
        content: A todo item tracks one task.
`

func generateProject(t *testing.T, concepts string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "noli.yaml"), []byte(generateProjectConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".noli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".noli", "concepts.yaml"), []byte(concepts), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGenerateDryRunCommand(t *testing.T) {
	dir := generateProject(t, generateProjectConcepts)
	exit, envelope, _, stderr := runCLI(t,
		"generate", "--config", filepath.Join(dir, "noli.yaml"), "--dry-run")
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	data := envelope["data"].(map[string]any)
	if data["mode"] != "dry-run" || data["preview_root"] != ".noli/preview" {
		t.Fatalf("data = %#v", data)
	}
	if len(data["added"].([]any)) != 3 { // concept, concepts/index, index
		t.Fatalf("added = %#v", data["added"])
	}
	if _, err := os.Stat(filepath.Join(dir, "knowledge")); !os.IsNotExist(err) {
		t.Fatal("dry-run created active knowledge")
	}
}

func TestGenerateApplyCommand(t *testing.T) {
	dir := generateProject(t, generateProjectConcepts)
	exit, envelope, _, _ := runCLI(t,
		"generate", "--config", filepath.Join(dir, "noli.yaml"), "--apply")
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	data := envelope["data"].(map[string]any)
	if data["mode"] != "apply" || data["preview_root"] != "" {
		t.Fatalf("data = %#v", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "knowledge", "concepts", "todo-item.md")); err != nil {
		t.Fatalf("applied document missing: %v", err)
	}
	exit, envelope, _, _ = runCLI(t, "validate", "--root", filepath.Join(dir, "knowledge"))
	if exit != 0 {
		t.Fatalf("applied bundle failed standard validation: %#v", envelope)
	}
}

func TestGenerateFlagValidation(t *testing.T) {
	dir := generateProject(t, generateProjectConcepts)
	config := filepath.Join(dir, "noli.yaml")
	cases := [][]string{
		{"generate", "--config", config},
		{"generate", "--config", config, "--dry-run", "--apply"},
		{"generate", "--dry-run"},
	}
	for _, args := range cases {
		exit, envelope, _, _ := runCLI(t, args...)
		if exit != 2 || errorCode(t, envelope) != "INVALID_ARGUMENT" {
			t.Fatalf("%v: exit %d (%#v)", args, exit, envelope)
		}
	}
	exit, envelope, _, _ := runCLI(t,
		"generate", "--config", filepath.Join(dir, "missing.yaml"), "--dry-run")
	if exit != 3 || errorCode(t, envelope) != "KNOWLEDGE_NOT_FOUND" {
		t.Fatalf("missing config: exit %d (%#v)", exit, envelope)
	}
}

func TestGenerateValidationFailureExitFour(t *testing.T) {
	broken := strings.Replace(generateProjectConcepts, "heading: Definition", "heading: Overview", 1)
	dir := generateProject(t, broken)
	exit, envelope, _, _ := runCLI(t,
		"generate", "--config", filepath.Join(dir, "noli.yaml"), "--apply")
	if exit != 4 || errorCode(t, envelope) != "VALIDATION_FAILED" {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	if _, err := os.Stat(filepath.Join(dir, "knowledge")); !os.IsNotExist(err) {
		t.Fatal("failed apply left knowledge behind")
	}
}

func TestGenerateGenerationFailureExitFive(t *testing.T) {
	broken := generateProjectConcepts + `    relationships:
      - predicate: applies-to
        to: concepts/missing
`
	dir := generateProject(t, broken)
	exit, envelope, _, _ := runCLI(t,
		"generate", "--config", filepath.Join(dir, "noli.yaml"), "--dry-run")
	if exit != 5 || errorCode(t, envelope) != "GENERATION_FAILED" {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
}
