package okf

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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
	bundle, err := ParseBundle(root)
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
}

func TestExtractLocalLinksResolutionAndFiltering(t *testing.T) {
	root := t.TempDir()
	body := `[relative](../products/x100.md#details)
[root](/policies/refund.md#exceptions)
[external](https://example.com/x.md)
[mail](mailto:help@example.com)
![image](../images/x.md)
[asset](manual.pdf)
[anchor](#local)`
	links, err := ExtractLocalLinks(root, "troubleshooting/wifi.md", body)
	if err != nil {
		t.Fatalf("ExtractLocalLinks() error = %v", err)
	}
	want := []string{"policies/refund", "products/x100"}
	if !reflect.DeepEqual(links, want) {
		t.Fatalf("links = %#v, want %#v", links, want)
	}
}

func TestExtractLocalLinksRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, destination := range []string{"../../../secret.md", "%2e%2e/%2e%2e/secret.md"} {
		_, err := ExtractLocalLinks(root, "docs/item.md", "[bad]("+destination+")")
		if err == nil || !strings.Contains(err.Error(), "escapes knowledge root") {
			t.Fatalf("destination %q error = %v", destination, err)
		}
	}
}

func TestParseDocumentRejectsPathOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeTestFile(t, outside, "---\ntype: Note\n---\n\nbody\n")
	if _, err := ParseDocument(root, outside); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("outside path error = %v", err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
