package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// driftGit runs one git command inside the test repository with a fixed
// identity so commits are deterministic and hermetic.
func driftGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	base := []string{"-C", dir,
		"-c", "user.name=noli-test", "-c", "user.email=noli@test.invalid",
		"-c", "commit.gpgsign=false"}
	command := exec.Command("git", append(base, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

// driftProject builds a generated project inside a fresh git repository with
// the knowledge bundle applied and everything committed.
func driftProject(t *testing.T) string {
	t.Helper()
	dir := generateProject(t, generateProjectConcepts)
	config := filepath.Join(dir, "noli.yaml")
	if exit, envelope, _, _ := runCLI(t, "generate", "--config", config, "--apply"); exit != 0 {
		t.Fatalf("apply failed: %#v", envelope)
	}
	driftGit(t, dir, "init", "-q")
	driftGit(t, dir, "add", ".")
	driftGit(t, dir, "commit", "-q", "-m", "knowledge baseline")
	return dir
}

func driftData(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing data: %#v", envelope)
	}
	return data
}

func TestDriftCleanProject(t *testing.T) {
	dir := driftProject(t)
	exit, envelope, _, stderr := runCLI(t, "drift", "--config", filepath.Join(dir, "noli.yaml"))
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	data := driftData(t, envelope)
	if data["drifted"] != false || data["git"] != "available" {
		t.Fatalf("data = %#v", data)
	}
	if data["baseline"] == "" {
		t.Fatal("baseline commit missing")
	}
	bundle := data["bundle"].(map[string]any)
	if bundle["in_sync"] != true {
		t.Fatalf("bundle = %#v", bundle)
	}
	if len(data["undocumented_files"].([]any)) != 0 {
		t.Fatalf("undocumented_files = %#v", data["undocumented_files"])
	}
}

func TestDriftDetectsHandEditedKnowledge(t *testing.T) {
	dir := driftProject(t)
	document := filepath.Join(dir, "knowledge", "concepts", "todo-item.md")
	if err := os.WriteFile(document, []byte("---\ntype: Domain Entity\ntitle: Todo Item\n---\n\nhand edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exit, envelope, _, _ := runCLI(t, "drift", "--config", filepath.Join(dir, "noli.yaml"))
	if exit != 4 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	data := driftData(t, envelope)
	bundle := data["bundle"].(map[string]any)
	if data["drifted"] != true || bundle["in_sync"] != false {
		t.Fatalf("data = %#v", data)
	}
	changed := bundle["changed"].([]any)
	if len(changed) != 1 || changed[0] != "concepts/todo-item" {
		t.Fatalf("changed = %#v", changed)
	}
	// The edited file lives under the knowledge root, so it must never also
	// surface as an undocumented repository change (porcelain lines start
	// with a significant space).
	if files := data["undocumented_files"].([]any); len(files) != 0 {
		t.Fatalf("undocumented_files = %#v", files)
	}
}

func TestDriftDetectsUndocumentedFiles(t *testing.T) {
	dir := driftProject(t)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("manual readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	driftGit(t, dir, "add", "main.py")
	driftGit(t, dir, "commit", "-q", "-m", "add program without knowledge update")

	exit, envelope, _, _ := runCLI(t, "drift", "--config", filepath.Join(dir, "noli.yaml"))
	if exit != 4 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	data := driftData(t, envelope)
	bundle := data["bundle"].(map[string]any)
	if bundle["in_sync"] != true {
		t.Fatalf("bundle = %#v", bundle)
	}
	files := data["undocumented_files"].([]any)
	if len(files) != 2 {
		t.Fatalf("undocumented_files = %#v", files)
	}
	first := files[0].(map[string]any)
	second := files[1].(map[string]any)
	if first["path"] != "README.md" || first["state"] != "untracked" {
		t.Fatalf("first = %#v", first)
	}
	if second["path"] != "main.py" || second["state"] != "added" {
		t.Fatalf("second = %#v", second)
	}
}

func TestDriftKnowledgeUpdateClearsCodeDrift(t *testing.T) {
	dir := driftProject(t)
	if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	driftGit(t, dir, "add", "main.py")
	driftGit(t, dir, "commit", "-q", "-m", "add program")
	// Documenting the change moves the baseline forward: commit touching the
	// concept source clears the previously undocumented commit.
	concepts := filepath.Join(dir, ".noli", "concepts.yaml")
	updated := generateProjectConcepts + "      - heading: Notes\n        content: Documented main.py.\n"
	if err := os.WriteFile(concepts, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	if exit, envelope, _, _ := runCLI(t, "generate", "--config", filepath.Join(dir, "noli.yaml"), "--apply"); exit != 0 {
		t.Fatalf("re-apply failed: %#v", envelope)
	}
	driftGit(t, dir, "add", ".")
	driftGit(t, dir, "commit", "-q", "-m", "document main.py")

	exit, envelope, _, _ := runCLI(t, "drift", "--config", filepath.Join(dir, "noli.yaml"))
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	if data := driftData(t, envelope); data["drifted"] != false {
		t.Fatalf("data = %#v", data)
	}
}

func TestDriftWithoutGitRepository(t *testing.T) {
	dir := generateProject(t, generateProjectConcepts)
	config := filepath.Join(dir, "noli.yaml")
	if exit, envelope, _, _ := runCLI(t, "generate", "--config", config, "--apply"); exit != 0 {
		t.Fatalf("apply failed: %#v", envelope)
	}
	exit, envelope, _, _ := runCLI(t, "drift", "--config", config)
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	data := driftData(t, envelope)
	if data["git"] != "unavailable" || data["baseline"] != "" || data["drifted"] != false {
		t.Fatalf("data = %#v", data)
	}
	if len(data["undocumented_files"].([]any)) != 0 {
		t.Fatalf("undocumented_files = %#v", data["undocumented_files"])
	}
}

func TestDriftFlagValidation(t *testing.T) {
	exit, envelope, _, _ := runCLI(t, "drift")
	if exit != 2 || errorCode(t, envelope) != "INVALID_ARGUMENT" {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	exit, envelope, _, _ = runCLI(t, "drift", "--config", filepath.Join(t.TempDir(), "missing.yaml"))
	if exit != 3 || errorCode(t, envelope) != "KNOWLEDGE_NOT_FOUND" {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
}
