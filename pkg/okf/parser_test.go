package okf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func conceptDocument(title string) string {
	return "---\ntype: Note\ntitle: " + title + "\n---\n\nBody of " + title + "\n"
}

func TestParseDocumentPreservesMetadataAndDetectsSpecialFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.md"), `---
type: Navigation
title: Home
custom:
  owner: docs
---

# Home
`)
	writeTestFile(t, filepath.Join(root, "log.md"), `---
type: Bundle Log
title: Log
---

# Log
`)
	bundle, err := ParseBundle(root, ParseOptions{})
	if err != nil {
		t.Fatalf("ParseBundle() error = %v", err)
	}
	index := bundle.Documents["index"]
	if !index.IsIndex || index.IsLog {
		t.Fatalf("index flags = index:%v log:%v", index.IsIndex, index.IsLog)
	}
	custom, ok := index.Metadata.Extra["custom"].(map[string]any)
	if !ok || custom["owner"] != "docs" {
		t.Fatalf("custom metadata = %#v", index.Metadata.Extra["custom"])
	}
	if !bundle.Documents["log"].IsLog {
		t.Fatal("root log.md was not detected")
	}
	if bundle.BundleID == "" || !strings.HasPrefix(bundle.BundleID, "sha256:") {
		t.Fatalf("BundleID = %q", bundle.BundleID)
	}
}

func TestMetadataRoundTripPreservesUnknownFields(t *testing.T) {
	original := Metadata{
		Type:  "Note",
		Title: "T",
		Tags:  []string{"a", "b"},
		Extra: map[string]any{"custom": map[string]any{"owner": "docs"}, "confidence": 0.9},
	}
	encoded, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded Metadata
	if err := yaml.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.Type != "Note" || decoded.Title != "T" || len(decoded.Tags) != 2 {
		t.Fatalf("decoded = %#v", decoded)
	}
	custom, ok := decoded.Extra["custom"].(map[string]any)
	if !ok || custom["owner"] != "docs" {
		t.Fatalf("Extra round trip = %#v", decoded.Extra)
	}
}

func TestParseDocumentRejectsInvalidFrontmatterAndEncoding(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing.md")
	writeTestFile(t, missing, "# no frontmatter\n")
	if _, err := ParseDocument(root, missing); err == nil || !strings.Contains(err.Error(), "missing opening") {
		t.Fatalf("missing frontmatter error = %v", err)
	}
	invalid := filepath.Join(root, "invalid.md")
	if err := os.WriteFile(invalid, []byte{0xff, 0xfe}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseDocument(root, invalid); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("invalid encoding error = %v", err)
	}
	multi := filepath.Join(root, "multi.md")
	writeTestFile(t, multi, "---\ntype: Note\n...\nmore: yaml\n---\n\nbody\n")
	if _, err := ParseDocument(root, multi); err == nil || !strings.Contains(err.Error(), "frontmatter") {
		t.Fatalf("multiple YAML documents error = %v", err)
	}
}

func TestParseBundleReportsStableProblemCodes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bad.md"), "no frontmatter\n")
	writeTestFile(t, filepath.Join(root, "list.md"), "---\n- not\n- a\n- mapping\n---\n\nbody\n")
	writeTestFile(t, filepath.Join(root, "a.md"), conceptDocument("A"))
	writeTestFile(t, filepath.Join(root, "a.MD"), conceptDocument("A2"))
	_, err := ParseBundle(root, ParseOptions{})
	var aggregate *ParseErrors
	if !errors.As(err, &aggregate) {
		t.Fatalf("ParseBundle() error = %v", err)
	}
	codes := make(map[string]int)
	for _, problem := range aggregate.Problems {
		codes[problem.Code]++
	}
	if codes[CodeParseError] != 1 || codes[CodeInvalidFrontmatter] != 1 || codes[CodeDuplicateID] != 1 {
		t.Fatalf("problem codes = %#v (%v)", codes, aggregate.Problems)
	}
}

func TestParseBundleSkipsSensitiveAndExcludedPaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "keep.md"), conceptDocument("Keep"))
	writeTestFile(t, filepath.Join(root, ".git", "hidden.md"), conceptDocument("Hidden"))
	writeTestFile(t, filepath.Join(root, "node_modules", "dep.md"), conceptDocument("Dep"))
	writeTestFile(t, filepath.Join(root, "drafts", "wip.md"), conceptDocument("WIP"))
	writeTestFile(t, filepath.Join(root, "notes", "old", "n.md"), conceptDocument("N"))
	bundle, err := ParseBundle(root, ParseOptions{Exclude: []string{"drafts", "notes/old"}})
	if err != nil {
		t.Fatalf("ParseBundle() error = %v", err)
	}
	if len(bundle.Order) != 1 || bundle.Order[0] != "keep" {
		t.Fatalf("Order = %#v", bundle.Order)
	}
}

func TestParseBundleEnforcesSizeAndCountBounds(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "small.md"), conceptDocument("Small"))
	writeTestFile(t, filepath.Join(root, "big.md"), "---\ntype: Note\n---\n\n"+strings.Repeat("x", 500)+"\n")
	_, err := ParseBundle(root, ParseOptions{MaxFileBytes: 100})
	var aggregate *ParseErrors
	if !errors.As(err, &aggregate) || len(aggregate.Problems) != 1 || aggregate.Problems[0].Code != CodeParseError {
		t.Fatalf("file bound error = %v", err)
	}
	if bundle, _ := ParseBundle(root, ParseOptions{MaxFileBytes: 100}); bundle == nil || len(bundle.Order) != 1 {
		t.Fatal("oversized file removed the whole bundle")
	}
	if _, err := ParseBundle(root, ParseOptions{MaxTotalBytes: 60}); err == nil || !strings.Contains(err.Error(), "total size limit") {
		t.Fatalf("total bound error = %v", err)
	}
	if _, err := ParseBundle(root, ParseOptions{MaxDocuments: 1}); err == nil || !strings.Contains(err.Error(), "document limit") {
		t.Fatalf("count bound error = %v", err)
	}
	if _, err := ParseBundle(root, ParseOptions{MaxDocuments: -1}); err == nil {
		t.Fatal("negative bounds were accepted")
	}
}

func TestParseBundleIgnoresSymlinksInside(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(root, "real.md"), conceptDocument("Real"))
	writeTestFile(t, filepath.Join(outside, "target.md"), conceptDocument("Target"))
	if err := os.Symlink(filepath.Join(outside, "target.md"), filepath.Join(root, "link.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linkdir")); err != nil {
		t.Fatal(err)
	}
	bundle, err := ParseBundle(root, ParseOptions{})
	if err != nil {
		t.Fatalf("ParseBundle() error = %v", err)
	}
	if len(bundle.Order) != 1 || bundle.Order[0] != "real" {
		t.Fatalf("Order = %#v", bundle.Order)
	}
}

func TestParseBundleResolvesSymlinkRoot(t *testing.T) {
	real := t.TempDir()
	writeTestFile(t, filepath.Join(real, "doc.md"), conceptDocument("Doc"))
	linkRoot := filepath.Join(t.TempDir(), "link-root")
	if err := os.Symlink(real, linkRoot); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	bundle, err := ParseBundle(linkRoot, ParseOptions{})
	if err != nil {
		t.Fatalf("ParseBundle() error = %v", err)
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Root != resolved {
		t.Fatalf("Root = %q, want %q", bundle.Root, resolved)
	}
}

func TestParseDocumentRejectsPathOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeTestFile(t, outside, "---\ntype: Note\n---\n\nbody\n")
	if _, err := ParseDocument(root, outside); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("outside path error = %v", err)
	}
	inside := filepath.Join(root, "link.md")
	if err := os.Symlink(outside, inside); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := ParseDocument(root, inside); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("symlink escape error = %v", err)
	}
}

func TestBundleIDIsDeterministicAndContentSensitive(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), conceptDocument("A"))
	first, err := ParseBundle(root, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseBundle(root, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if first.BundleID != second.BundleID {
		t.Fatalf("BundleID not deterministic: %q vs %q", first.BundleID, second.BundleID)
	}
	writeTestFile(t, filepath.Join(root, "a.md"), conceptDocument("Changed"))
	third, err := ParseBundle(root, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if third.BundleID == first.BundleID {
		t.Fatal("BundleID did not change with content")
	}
}
