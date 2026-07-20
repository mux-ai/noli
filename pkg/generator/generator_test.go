package generator

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"noli/pkg/internal/targetlock"
	"noli/pkg/okf"
)

const generatorConfig = `version: 1
project:
  name: Todo App
knowledge:
  root: knowledge
concept_types:
  - type: Domain Entity
    directory: concepts
    required_sections: [Definition]
  - type: Business Rule
    directory: rules
relationships:
  - predicate: applies-to
    from: Business Rule
    to: Domain Entity
  - predicate: uses
generation:
  concept_files: [.noli/concepts.yaml]
`

const generatorConcepts = `concepts:
  - type: Domain Entity
    title: Todo Item
    description: The core task entity.
    tags: [domain]
    attributes:
      severity: must
    sections:
      - heading: Definition
        content: A todo item tracks one task.
  - type: Business Rule
    title: Complete Task
    description: Rules for completing a task.
    sections:
      - heading: Statement
        content: A task may only be completed once.
    relationships:
      - predicate: applies-to
        to: Todo Item
    citations:
      - source: spec.md
        uri: https://example.com/spec.md
        page: 3
        evidence: completion rules
`

func generatorProject(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "noli.yaml"), []byte(generatorConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".noli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".noli", "concepts.yaml"), []byte(generatorConcepts), 0o644); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig(filepath.Join(dir, "noli.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return config
}

// snapshotTree maps relative paths to content hashes below root.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(rel)] = contentHash(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestGenerateDryRunIsDeterministicAndLeavesActiveUntouched(t *testing.T) {
	config := generatorProject(t)
	result, err := Generate(config, GenerateOptions{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Mode != "dry-run" || result.PreviewRoot != DefaultPreviewDir {
		t.Fatalf("result = %#v", result)
	}
	wantAdded := []string{
		"concepts/index", "concepts/todo-item", "index",
		"rules/complete-task", "rules/index",
	}
	if !reflect.DeepEqual(result.Added, wantAdded) {
		t.Fatalf("Added = %#v, want %#v", result.Added, wantAdded)
	}
	if len(result.Changed)+len(result.Removed)+len(result.Unchanged) != 0 {
		t.Fatalf("unexpected diff: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(config.Dir(), "knowledge")); !os.IsNotExist(err) {
		t.Fatal("dry-run created the active knowledge root")
	}

	preview := filepath.Join(config.Dir(), filepath.FromSlash(DefaultPreviewDir))
	first := snapshotTree(t, preview)
	if _, err := Generate(config, GenerateOptions{}); err != nil {
		t.Fatal(err)
	}
	second := snapshotTree(t, preview)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("two identical dry-runs differ:\n%v\n%v", first, second)
	}
}

func TestGenerateApplyProducesValidRoundTrippableBundle(t *testing.T) {
	config := generatorProject(t)
	result, err := Generate(config, GenerateOptions{Apply: true})
	if err != nil {
		t.Fatalf("Generate(apply) error = %v", err)
	}
	if result.Mode != "apply" || result.PreviewRoot != "" || len(result.Added) != 5 {
		t.Fatalf("result = %#v", result)
	}
	root := filepath.Join(config.Dir(), "knowledge")
	report := okf.Validate(root, config.ValidationOptions())
	if !report.Valid {
		t.Fatalf("applied bundle invalid: %#v", report.Errors)
	}
	store, err := okf.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	rule, ok := store.Get("rules/complete-task")
	if !ok {
		t.Fatal("rules/complete-task missing")
	}
	// The rendered relationship phrase must round-trip into a typed link.
	found := false
	for _, link := range rule.Links {
		if link.Target == "concepts/todo-item" && link.Predicate == "applies-to" {
			found = true
		}
	}
	if !found {
		t.Fatalf("typed relationship did not round-trip: %#v", rule.Links)
	}
	if !strings.Contains(rule.Body, "## Citations") ||
		!strings.Contains(rule.Body, "[1] [spec.md](<https://example.com/spec.md>), page 3") {
		t.Fatalf("citations missing: %q", rule.Body)
	}

	again, err := Generate(config, GenerateOptions{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Unchanged) != 5 || len(again.Added)+len(again.Changed)+len(again.Removed) != 0 {
		t.Fatalf("second apply diff = %#v", again)
	}
}

func TestGenerateApplyRollsBackOnValidationFailure(t *testing.T) {
	config := generatorProject(t)
	if _, err := Generate(config, GenerateOptions{Apply: true}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(config.Dir(), "knowledge")
	before := snapshotTree(t, root)

	// Remove the required Definition section so project validation fails.
	broken := strings.Replace(generatorConcepts, "heading: Definition", "heading: Overview", 1)
	if err := os.WriteFile(filepath.Join(config.Dir(), ".noli", "concepts.yaml"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Generate(config, GenerateOptions{Apply: true})
	var validation *BundleValidationError
	if !errors.As(err, &validation) || len(validation.Errors) == 0 {
		t.Fatalf("error = %v, want BundleValidationError", err)
	}
	after := snapshotTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed apply modified the active knowledge")
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(root), "."+filepath.Base(root)+".applying-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("leftover apply paths = %v, error = %v", leftovers, err)
	}
}

func TestGenerateRejectsConcurrentWriter(t *testing.T) {
	config := generatorProject(t)
	lock, err := targetlock.Acquire(filepath.Join(config.Dir(), ".noli", "write.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()

	_, err = Generate(config, GenerateOptions{Apply: true})
	if !errors.Is(err, ErrWriteBusy) {
		t.Fatalf("Generate() error = %v, want ErrWriteBusy", err)
	}
}

func TestGenerateDiffDetectsChangesAndRemovals(t *testing.T) {
	config := generatorProject(t)
	if _, err := Generate(config, GenerateOptions{Apply: true}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(config.Dir(), "knowledge")
	// A manually added document should be reported as removed.
	if err := os.WriteFile(filepath.Join(root, "concepts", "manual.md"),
		[]byte("---\ntype: Domain Entity\ntitle: Manual\n---\n\n## Definition\n\nmanual\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(generatorConcepts, "The core task entity.", "The changed task entity.", 1)
	if err := os.WriteFile(filepath.Join(config.Dir(), ".noli", "concepts.yaml"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Generate(config, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantChanged := []string{"concepts/index", "concepts/todo-item"}
	if !reflect.DeepEqual(result.Changed, wantChanged) {
		t.Fatalf("Changed = %#v, want %#v", result.Changed, wantChanged)
	}
	if !reflect.DeepEqual(result.Removed, []string{"concepts/manual"}) {
		t.Fatalf("Removed = %#v", result.Removed)
	}
}

func TestGenerateInputValidationErrors(t *testing.T) {
	cases := []struct {
		name     string
		concepts string
		message  string
	}{
		{"unknown type", "concepts:\n  - type: Mystery\n    title: X\n", "not configured"},
		{"missing title", "concepts:\n  - type: Domain Entity\n", "title is required"},
		{"unresolved relationship", strings.Replace(generatorConcepts, "to: Todo Item", "to: concepts/missing", 1),
			`relationship target "concepts/missing" cannot be resolved`},
		{"predicate not allowed", strings.Replace(generatorConcepts, "predicate: applies-to", "predicate: mystery-verb", 1),
			"not configured in noli.yaml relationships"},
		{"duplicate ID", generatorConcepts + "  - type: Domain Entity\n    title: Todo Item\n    sections:\n      - heading: Definition\n        content: again\n",
			"duplicates"},
		{"reserved attribute", strings.Replace(generatorConcepts, "severity: must", "title: shadowed", 1),
			"reserved metadata field"},
		{"no inputs", "concepts: []\n", "no structured concept inputs"},
	}
	for _, testCase := range cases {
		config := generatorProject(t)
		if err := os.WriteFile(filepath.Join(config.Dir(), ".noli", "concepts.yaml"),
			[]byte(testCase.concepts), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Generate(config, GenerateOptions{})
		if err == nil || !strings.Contains(err.Error(), testCase.message) {
			t.Fatalf("%s: error = %v, want containing %q", testCase.name, err, testCase.message)
		}
	}
}

func TestGenerateNoInputsReRendersActiveBundle(t *testing.T) {
	config := generatorProject(t)
	if _, err := Generate(config, GenerateOptions{Apply: true}); err != nil {
		t.Fatal(err)
	}
	// Reload the config without any generation inputs.
	noGeneration := strings.Replace(generatorConfig,
		"generation:\n  concept_files: [.noli/concepts.yaml]\n", "", 1)
	if err := os.WriteFile(filepath.Join(config.Dir(), "noli.yaml"), []byte(noGeneration), 0o644); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadConfig(filepath.Join(config.Dir(), "noli.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Generate(reloaded, GenerateOptions{})
	if err != nil {
		t.Fatalf("no-input dry-run error = %v", err)
	}
	if result.Mode != "dry-run" || len(result.Unchanged) != 5 ||
		len(result.Added)+len(result.Changed)+len(result.Removed) != 0 {
		t.Fatalf("passthrough diff = %#v", result)
	}

	// A CRLF file is reported as changed and normalized by apply.
	root := filepath.Join(reloaded.Dir(), "knowledge")
	path := filepath.Join(root, "concepts", "todo-item.md")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	crlf := strings.ReplaceAll(string(original), "\n", "\r\n")
	if err := os.WriteFile(path, []byte(crlf), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := Generate(reloaded, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(changed.Changed, []string{"concepts/todo-item"}) {
		t.Fatalf("CRLF diff = %#v", changed)
	}
	if _, err := Generate(reloaded, GenerateOptions{Apply: true}); err != nil {
		t.Fatal(err)
	}
	normalized, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(normalized) != string(original) {
		t.Fatal("apply did not normalize the CRLF file")
	}
}

// OKF v0.1 section 6: index files carry no frontmatter and list concepts as
// `* [Title](link) - description`; section 5.1 recommends absolute
// bundle-relative links.
func TestGeneratedBundleIsSpecConformant(t *testing.T) {
	config := generatorProject(t)
	if _, err := Generate(config, GenerateOptions{Apply: true}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(config.Dir(), "knowledge")
	for _, index := range []string{"index.md", filepath.Join("concepts", "index.md")} {
		data, err := os.ReadFile(filepath.Join(root, index))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(string(data), "---") {
			t.Fatalf("%s carries frontmatter:\n%s", index, data)
		}
		if !strings.Contains(string(data), "* [") {
			t.Fatalf("%s does not use the section 6 bullet form:\n%s", index, data)
		}
	}
	rootIndex, err := os.ReadFile(filepath.Join(root, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rootIndex), "](/concepts/index.md)") {
		t.Fatalf("root index does not use absolute bundle links:\n%s", rootIndex)
	}
	rule, err := os.ReadFile(filepath.Join(root, "rules", "complete-task.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rule), "](/concepts/todo-item.md)") {
		t.Fatalf("relationship link is not absolute:\n%s", rule)
	}
	if !strings.Contains(string(rule), "[1] [spec.md](<https://example.com/spec.md>)") {
		t.Fatalf("citation does not use the section 8 numbered-link form:\n%s", rule)
	}
	if _, err := os.Stat(filepath.Join(root, "log.md")); !os.IsNotExist(err) {
		t.Fatal("generation invented a log.md")
	}
	// The generated bundle must still validate and round-trip typed links.
	if report := okf.Validate(root, config.ValidationOptions()); !report.Valid {
		t.Fatalf("generated bundle invalid: %#v", report.Errors)
	}
	store, err := okf.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	document, _ := store.Get("rules/complete-task")
	found := false
	for _, link := range document.Links {
		if link.Target == "concepts/todo-item" && link.Predicate == "applies-to" {
			found = true
		}
	}
	if !found {
		t.Fatalf("absolute typed link did not round-trip: %#v", document.Links)
	}
}

// A hand-authored log.md must survive apply even though generation never
// writes one.
func TestGenerateReservesHandAuthoredLogs(t *testing.T) {
	config := generatorProject(t)
	if _, err := Generate(config, GenerateOptions{Apply: true}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(config.Dir(), "knowledge")
	logBody := "# Log\n\n## 2026-07-20\n\n**Creation** Initial bundle.\n"
	if err := os.WriteFile(filepath.Join(root, "log.md"), []byte(logBody), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Generate(config, GenerateOptions{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 0 {
		t.Fatalf("apply removed documents: %#v", result.Removed)
	}
	preserved, err := os.ReadFile(filepath.Join(root, "log.md"))
	if err != nil || string(preserved) != logBody {
		t.Fatalf("hand-authored log was not preserved: %v %q", err, preserved)
	}
}

func TestGeneratePreviewMustNotOverlapKnowledge(t *testing.T) {
	config := generatorProject(t)
	_, err := Generate(config, GenerateOptions{PreviewDir: "knowledge"})
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("error = %v, want ErrUnsafePath", err)
	}
	_, err = Generate(config, GenerateOptions{PreviewDir: "knowledge/preview"})
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("nested preview error = %v, want ErrUnsafePath", err)
	}
}

func TestSlugAndCanonicalIDs(t *testing.T) {
	for input, expected := range map[string]string{
		" Wi-Fi  Failure! ": "wi-fi-failure",
		"Crème brûlée":      "crème-brûlée",
		"产品 100":            "产品-100",
	} {
		actual, err := Slug(input)
		if err != nil || actual != expected {
			t.Fatalf("Slug(%q) = %q, %v; want %q", input, actual, err, expected)
		}
	}
	if _, err := Slug("../"); err == nil {
		t.Fatal("unsafe slug accepted")
	}
	rule := ConceptTypeConfig{Type: "Domain Entity", Directory: "concepts"}
	if id, err := canonicalConceptID(rule, ConceptInput{Title: "Todo Item"}); err != nil || id != "concepts/todo-item" {
		t.Fatalf("canonicalConceptID = %q, %v", id, err)
	}
	if _, err := canonicalConceptID(rule, ConceptInput{ID: "rules/todo-item", Title: "X"}); err == nil {
		t.Fatal("wrong-directory ID accepted")
	}
	if _, err := canonicalConceptID(rule, ConceptInput{ID: "concepts/index", Title: "X"}); err == nil {
		t.Fatal("reserved ID accepted")
	}
	if _, err := canonicalConceptID(rule, ConceptInput{Title: "Index"}); err == nil {
		t.Fatal("reserved slug accepted")
	}
}
