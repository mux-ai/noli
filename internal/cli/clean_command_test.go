package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func cleanProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if exit, envelope, _, _ := runCLI(t, "enable", "--dir", dir); exit != 0 {
		t.Fatalf("enable failed: %#v", envelope)
	}
	return dir
}

func TestCleanPreviewListsWithoutDeleting(t *testing.T) {
	dir := cleanProject(t)
	exit, envelope, _, stderr := runCLI(t, "clean", "--dir", dir)
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	data := envelope["data"].(map[string]any)
	if data["mode"] != "preview" || data["changed"] != false {
		t.Fatalf("data = %#v", data)
	}
	removed := data["removed"].([]any)
	if len(removed) != 3 || removed[0] != ".noli" || removed[1] != "knowledge" || removed[2] != "noli.yaml" {
		t.Fatalf("removed = %#v", removed)
	}
	for _, path := range []string{"noli.yaml", ".noli", "knowledge"} {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatalf("preview deleted %s: %v", path, err)
		}
	}
}

func TestCleanForceDeletesEverything(t *testing.T) {
	dir := cleanProject(t)
	if err := os.WriteFile(filepath.Join(dir, "noli-agent-queries.yaml"), []byte("queries: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('kept')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exit, envelope, _, _ := runCLI(t, "clean", "--dir", dir, "--force")
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	data := envelope["data"].(map[string]any)
	if data["mode"] != "clean" || data["changed"] != true {
		t.Fatalf("data = %#v", data)
	}
	if len(data["removed"].([]any)) != 4 {
		t.Fatalf("removed = %#v", data["removed"])
	}
	for _, path := range []string{"noli.yaml", ".noli", "knowledge", "noli-agent-queries.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, path)); !os.IsNotExist(err) {
			t.Fatalf("%s survived clean --force", path)
		}
	}
	// Non-Noli files are never touched.
	if _, err := os.Stat(filepath.Join(dir, "main.py")); err != nil {
		t.Fatal("clean deleted an unrelated project file")
	}
	// Second force run: nothing left, changed false.
	exit, envelope, _, _ = runCLI(t, "clean", "--dir", dir, "--force")
	if exit != 0 {
		t.Fatalf("second exit = %d (%#v)", exit, envelope)
	}
	data = envelope["data"].(map[string]any)
	if data["changed"] != false || len(data["removed"].([]any)) != 0 {
		t.Fatalf("second run data = %#v", data)
	}
}

func TestCleanRemovesLegacyAndCustomRoots(t *testing.T) {
	dir := t.TempDir()
	// Custom knowledge root via config plus legacy okf remnants.
	config := "version: 1\nproject:\n  name: Custom\nknowledge:\n  root: docs/kb\n" +
		"concept_types:\n  - type: Domain Entity\n    directory: concepts\n"
	if err := os.WriteFile(filepath.Join(dir, "noli.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"docs/kb", ".okf"} {
		if err := os.MkdirAll(filepath.Join(dir, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "okf.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exit, envelope, _, _ := runCLI(t, "clean", "--dir", dir, "--force")
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	removed := envelope["data"].(map[string]any)["removed"].([]any)
	if len(removed) != 4 || removed[0] != ".okf" || removed[1] != "docs/kb" ||
		removed[2] != "noli.yaml" || removed[3] != "okf.yaml" {
		t.Fatalf("removed = %#v", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "kb")); !os.IsNotExist(err) {
		t.Fatal("custom knowledge root survived")
	}
	// The parent of a custom root is preserved.
	if _, err := os.Stat(filepath.Join(dir, "docs")); err != nil {
		t.Fatal("clean deleted the custom root's parent directory")
	}
}

func TestCleanFlagValidation(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	exit, envelope, _, _ := runCLI(t, "clean", "--dir", missing)
	if exit != 3 || errorCode(t, envelope) != "KNOWLEDGE_NOT_FOUND" {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	exit, envelope, _, _ = runCLI(t, "clean", "--dir", "")
	if exit != 2 || errorCode(t, envelope) != "INVALID_ARGUMENT" {
		t.Fatalf("empty dir: exit %d (%#v)", exit, envelope)
	}
}
