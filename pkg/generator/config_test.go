package generator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validConfig = `version: 1
project:
  name: Todo App
  description: Demonstration project
knowledge:
  root: knowledge
concept_types:
  - type: Domain Entity
    directory: concepts
    required_fields: [title]
    required_sections: [Definition]
    identity_fields: [title]
  - type: Business Rule
    aliases: [Rule]
    directory: rules
relationships:
  - predicate: applies-to
    from: Business Rule
    to: Domain Entity
retrieval:
  max_documents: 10
  max_characters: 12000
security:
  exclude: [drafts]
generation:
  concept_files: [.noli/concepts.yaml]
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "noli.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "knowledge"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigValid(t *testing.T) {
	path := writeConfig(t, validConfig)
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if config.Project.Name != "Todo App" || len(config.ConceptTypes) != 2 {
		t.Fatalf("config = %#v", config)
	}
	root, err := config.KnowledgeRoot()
	if err != nil {
		t.Fatalf("KnowledgeRoot() error = %v", err)
	}
	resolvedDir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if root != filepath.Join(resolvedDir, "knowledge") {
		t.Fatalf("KnowledgeRoot() = %q", root)
	}
}

func TestLoadConfigAcceptsExplicitLegacyFilename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "okf.yaml")
	if err := os.WriteFile(path, []byte(validConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "knowledge"), 0o755); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("legacy LoadConfig() error = %v", err)
	}
	if config.Project.Name != "Todo App" {
		t.Fatalf("legacy config = %#v", config)
	}
}

func TestLoadConfigPrefersNoliFilename(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, LegacyConfigName)
	primary := filepath.Join(dir, PrimaryConfigName)
	if err := os.WriteFile(legacy, []byte(validConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	primaryConfig := strings.Replace(validConfig, "name: Todo App", "name: Noli Project", 1)
	if err := os.WriteFile(primary, []byte(primaryConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "knowledge"), 0o755); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if config.Project.Name != "Noli Project" {
		t.Fatalf("primary config did not win: %#v", config.Project)
	}
}

func TestLoadConfigRejectsUnknownFieldsAndBadValues(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(string) string
		message string
	}{
		{"unknown field", func(c string) string { return c + "surprise: true\n" }, "field surprise not found"},
		{"wrong version", func(c string) string { return strings.Replace(c, "version: 1", "version: 2", 1) }, "version must be 1"},
		{"missing project name", func(c string) string { return strings.Replace(c, "  name: Todo App\n", "", 1) }, "project.name is required"},
		{"missing knowledge root", func(c string) string { return strings.Replace(c, "  root: knowledge\n", "  root: \"\"\n", 1) }, "knowledge.root is required"},
		{"no concept types", func(c string) string {
			return strings.Replace(c, `concept_types:
  - type: Domain Entity
    directory: concepts
    required_fields: [title]
    required_sections: [Definition]
    identity_fields: [title]
  - type: Business Rule
    aliases: [Rule]
    directory: rules
`, "concept_types: []\n", 1)
		}, "at least one type"},
		{"duplicate alias", func(c string) string { return strings.Replace(c, "aliases: [Rule]", "aliases: [Domain Entity]", 1) }, "duplicates"},
		{"empty predicate", func(c string) string { return strings.Replace(c, "predicate: applies-to", "predicate: \"\"", 1) }, "predicate is required"},
		{"negative retrieval", func(c string) string { return strings.Replace(c, "max_documents: 10", "max_documents: -1", 1) }, "must not be negative"},
		{"bad minimum confidence", func(c string) string {
			return strings.Replace(c, "  description: Demonstration project\n", "  minimum_confidence: 3\n", 1)
		}, "minimum_confidence"},
	}
	for _, testCase := range cases {
		path := writeConfig(t, testCase.mutate(validConfig))
		_, err := LoadConfig(path)
		if err == nil || !strings.Contains(err.Error(), testCase.message) {
			t.Fatalf("%s: error = %v, want containing %q", testCase.name, err, testCase.message)
		}
	}
}

func TestLoadConfigRejectsUnsafePaths(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(string) string
	}{
		{"absolute root", func(c string) string { return strings.Replace(c, "root: knowledge", "root: /etc/knowledge", 1) }},
		{"traversal root", func(c string) string { return strings.Replace(c, "root: knowledge", "root: ../outside", 1) }},
		{"backslash root", func(c string) string { return strings.Replace(c, "root: knowledge", `root: "knowledge\\sub"`, 1) }},
		{"sensitive root", func(c string) string { return strings.Replace(c, "root: knowledge", "root: .git/knowledge", 1) }},
		{"sensitive concept file", func(c string) string {
			return strings.Replace(c, "concept_files: [.noli/concepts.yaml]", "concept_files: [../outside.yaml]", 1)
		}},
		{"absolute exclusion", func(c string) string { return strings.Replace(c, "exclude: [drafts]", "exclude: [/etc]", 1) }},
		{"traversal exclusion", func(c string) string { return strings.Replace(c, "exclude: [drafts]", "exclude: [../up]", 1) }},
	}
	for _, testCase := range cases {
		path := writeConfig(t, testCase.mutate(validConfig))
		_, err := LoadConfig(path)
		if !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("%s: error = %v, want ErrUnsafePath", testCase.name, err)
		}
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "noli.yaml"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestKnowledgeRootSymlinkContainment(t *testing.T) {
	// Contained symlink: knowledge -> real directory inside the project.
	containedDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(containedDir, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(containedDir, "noli.yaml"), []byte(validConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(containedDir, "real"), filepath.Join(containedDir, "knowledge")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	config, err := LoadConfig(filepath.Join(containedDir, "noli.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.KnowledgeRoot(); err != nil {
		t.Fatalf("contained symlink rejected: %v", err)
	}

	// Escaping symlink: knowledge -> directory outside the project.
	escapingDir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(escapingDir, "noli.yaml"), []byte(validConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(escapingDir, "knowledge")); err != nil {
		t.Fatal(err)
	}
	escapingConfig, err := LoadConfig(filepath.Join(escapingDir, "noli.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := escapingConfig.KnowledgeRoot(); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("escaping symlink error = %v, want ErrUnsafePath", err)
	}
}

func TestValidationOptionsMapping(t *testing.T) {
	content := strings.Replace(validConfig,
		"  description: Demonstration project\n",
		"  required_metadata: [description]\n  require_citations: true\n  require_confidence: true\n  minimum_confidence: 0.5\n", 1)
	config, err := LoadConfig(writeConfig(t, content))
	if err != nil {
		t.Fatal(err)
	}
	options := config.ValidationOptions()
	rules := options.Project
	if rules == nil || !rules.RequireCitations || !rules.RequireConfidence || rules.MinimumConfidence != 0.5 {
		t.Fatalf("rules = %#v", rules)
	}
	if len(rules.ConceptTypes) != 2 || rules.ConceptTypes[0].Directory != "concepts" {
		t.Fatalf("concept types = %#v", rules.ConceptTypes)
	}
	if len(rules.RequiredMetadata) != 1 || rules.RequiredMetadata[0] != "description" {
		t.Fatalf("required metadata = %#v", rules.RequiredMetadata)
	}
	if len(options.Parse.Exclude) != 1 || options.Parse.Exclude[0] != "drafts" {
		t.Fatalf("exclusions = %#v", options.Parse.Exclude)
	}
}
