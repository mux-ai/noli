package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnableBootstrapsEmptyRepository(t *testing.T) {
	dir := t.TempDir()
	exit, envelope, _, stderr := runCLI(t, "enable", "--dir", dir)
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	data := envelope["data"].(map[string]any)
	if data["state"] != "enabled" || data["changed"] != true || data["generated"] != true {
		t.Fatalf("data = %#v", data)
	}
	created := data["created"].([]any)
	if len(created) != 2 || created[0] != "noli.yaml" || created[1] != ".noli/concepts.yaml" {
		t.Fatalf("created = %#v", created)
	}
	if _, err := os.Stat(filepath.Join(dir, "knowledge", "index.md")); err != nil {
		t.Fatalf("bootstrap did not generate knowledge: %v", err)
	}
	exit, envelope, _, _ = runCLI(t,
		"validate", "--root", filepath.Join(dir, "knowledge"),
		"--mode", "project", "--config", filepath.Join(dir, "noli.yaml"))
	if exit != 0 {
		t.Fatalf("bootstrapped bundle failed validation: %#v", envelope)
	}
}

func TestEnableRemovesSentinelsAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	for _, sentinel := range []string{".noli", ".okf"} {
		if err := os.MkdirAll(filepath.Join(dir, sentinel), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, sentinel, "disabled"), []byte("developer opted out\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	exit, envelope, _, _ := runCLI(t, "enable", "--dir", dir)
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	data := envelope["data"].(map[string]any)
	removed := data["removed_sentinels"].([]any)
	if len(removed) != 2 || removed[0] != ".noli/disabled" || removed[1] != ".okf/disabled" {
		t.Fatalf("removed_sentinels = %#v", removed)
	}
	// Second run: everything already enabled, nothing to do.
	exit, envelope, _, _ = runCLI(t, "enable", "--dir", dir)
	if exit != 0 {
		t.Fatalf("second exit = %d (%#v)", exit, envelope)
	}
	data = envelope["data"].(map[string]any)
	if data["changed"] != false || data["generated"] != false {
		t.Fatalf("second run data = %#v", data)
	}
	if len(data["created"].([]any)) != 0 || len(data["removed_sentinels"].([]any)) != 0 {
		t.Fatalf("second run mutated state: %#v", data)
	}
}

func TestEnableKeepsExistingConfigAndConcepts(t *testing.T) {
	dir := generateProject(t, generateProjectConcepts)
	before, err := os.ReadFile(filepath.Join(dir, "noli.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	exit, envelope, _, _ := runCLI(t, "enable", "--dir", dir)
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	data := envelope["data"].(map[string]any)
	if len(data["created"].([]any)) != 0 || data["generated"] != true {
		t.Fatalf("data = %#v", data)
	}
	after, err := os.ReadFile(filepath.Join(dir, "noli.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("enable rewrote an existing noli.yaml")
	}
	if _, err := os.Stat(filepath.Join(dir, "knowledge", "concepts", "todo-item.md")); err != nil {
		t.Fatalf("existing concepts were not generated: %v", err)
	}
}

func TestDisableWritesSentinelAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	exit, envelope, _, _ := runCLI(t, "disable", "--dir", dir)
	if exit != 0 {
		t.Fatalf("exit = %d (%#v)", exit, envelope)
	}
	data := envelope["data"].(map[string]any)
	if data["state"] != "disabled" || data["changed"] != true || data["sentinel"] != ".noli/disabled" {
		t.Fatalf("data = %#v", data)
	}
	content, err := os.ReadFile(filepath.Join(dir, ".noli", "disabled"))
	if err != nil || string(content) != "developer opted out\n" {
		t.Fatalf("sentinel = %q, %v", content, err)
	}
	exit, envelope, _, _ = runCLI(t, "disable", "--dir", dir)
	if exit != 0 {
		t.Fatalf("second exit = %d (%#v)", exit, envelope)
	}
	if envelope["data"].(map[string]any)["changed"] != false {
		t.Fatalf("second run data = %#v", envelope["data"])
	}
}

func TestEnableDisableRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if exit, envelope, _, _ := runCLI(t, "disable", "--dir", dir); exit != 0 {
		t.Fatalf("disable: %#v", envelope)
	}
	if exit, envelope, _, _ := runCLI(t, "enable", "--dir", dir); exit != 0 {
		t.Fatalf("enable: %#v", envelope)
	}
	if _, err := os.Stat(filepath.Join(dir, ".noli", "disabled")); !os.IsNotExist(err) {
		t.Fatal("sentinel survived enable")
	}
	if _, err := os.Stat(filepath.Join(dir, "knowledge")); err != nil {
		t.Fatal("enable after disable did not bootstrap")
	}
}

func TestEnableDisableFlagValidation(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, command := range []string{"enable", "disable"} {
		exit, envelope, _, _ := runCLI(t, command, "--dir", missing)
		if exit != 3 || errorCode(t, envelope) != "KNOWLEDGE_NOT_FOUND" {
			t.Fatalf("%s: exit %d (%#v)", command, exit, envelope)
		}
		exit, envelope, _, _ = runCLI(t, command, "--dir", "")
		if exit != 2 || errorCode(t, envelope) != "INVALID_ARGUMENT" {
			t.Fatalf("%s empty dir: exit %d (%#v)", command, exit, envelope)
		}
	}
}

// TestStarterTemplatesMatchShared pins the embedded bootstrap templates to
// the shared integration copies so the CLI and the agent integrations can
// never bootstrap different starter knowledge.
func TestStarterTemplatesMatchShared(t *testing.T) {
	for _, name := range []string{"noli-starter.yaml", "noli-starter-concepts.yaml"} {
		embedded, err := starterTemplates.ReadFile("starter/" + name)
		if err != nil {
			t.Fatal(err)
		}
		shared, err := os.ReadFile(filepath.Join("..", "..", "integrations", "shared", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(embedded) != string(shared) {
			t.Fatalf("embedded %s differs from integrations/shared copy; re-copy it", name)
		}
	}
}
