package studio

import "testing"

func TestRuntimeConfigPrefersNoliEnvironment(t *testing.T) {
	t.Setenv("NOLI_WORKSPACE_PATH", "noli-workspace")
	t.Setenv("OKF_WORKSPACE_PATH", "legacy-workspace")
	t.Setenv("NOLI_MAX_CHUNK_CHARS", "2345")
	t.Setenv("OKF_MAX_CHUNK_CHARS", "1234")
	t.Setenv("NOLI_CONTEXT_LIMIT", "4567")
	t.Setenv("OKF_CONTEXT_LIMIT", "3456")
	t.Setenv("NOLI_REQUEST_TIMEOUT", "9s")
	t.Setenv("OKF_REQUEST_TIMEOUT", "8s")

	config, err := RuntimeConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if config.WorkspacePath != "noli-workspace" || config.MaximumChunk != 2345 ||
		config.ContextLimit != 4567 || config.RequestWait.String() != "9s" {
		t.Fatalf("Noli environment did not win: %#v", config)
	}
}

func TestRuntimeConfigAcceptsLegacyEnvironment(t *testing.T) {
	t.Setenv("NOLI_WORKSPACE_PATH", "")
	t.Setenv("NOLI_MAX_CHUNK_CHARS", "")
	t.Setenv("NOLI_CONTEXT_LIMIT", "")
	t.Setenv("NOLI_REQUEST_TIMEOUT", "")
	t.Setenv("OKF_WORKSPACE_PATH", "legacy-workspace")
	t.Setenv("OKF_MAX_CHUNK_CHARS", "1234")
	t.Setenv("OKF_CONTEXT_LIMIT", "3456")
	t.Setenv("OKF_REQUEST_TIMEOUT", "8s")

	config, err := RuntimeConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if config.WorkspacePath != "legacy-workspace" || config.MaximumChunk != 1234 ||
		config.ContextLimit != 3456 || config.RequestWait.String() != "8s" {
		t.Fatalf("legacy environment was not accepted: %#v", config)
	}
}
