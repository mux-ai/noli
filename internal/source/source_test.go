package source

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLoadDirectoryRecursivelyAndWarnsUnsupported(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "guide.md"), "# Setup\n\nInstall it.\n")
	mustWrite(t, filepath.Join(root, "nested", "data.json"), `{"z":1,"a":2}`)
	mustWrite(t, filepath.Join(root, "nested", "notes.xyz"), "ignored")

	loader := NewLoader()
	first, err := loader.LoadDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loader.LoadDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Documents) != 2 || len(first.Warnings) != 1 {
		t.Fatalf("LoadDirectory returned %d documents and %d warnings", len(first.Documents), len(first.Warnings))
	}
	if !strings.Contains(first.Warnings[0].Message, "unsupported") || first.Warnings[0].Path != "nested/notes.xyz" {
		t.Fatalf("warning = %#v", first.Warnings[0])
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("recursive loading is not deterministic:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if first.Documents[0].ID > first.Documents[1].ID {
		t.Fatal("documents are not ordered by stable ID")
	}
	for _, document := range first.Documents {
		if document.SourceURI == "" || filepath.IsAbs(document.SourceURI) {
			t.Errorf("unsafe or empty source URI: %q", document.SourceURI)
		}
		if document.ID != StableSourceID(document.SourceURI) {
			t.Errorf("source ID %q is not stable for %q", document.ID, document.SourceURI)
		}
	}
}

func TestMarkdownSectionsAndFenceHandling(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "guide.md")
	mustWrite(t, filename, "intro\n\n# First\nbody\n```\n# not a heading\n```\n## Second\nlast\n")
	document, err := NewLoader().LoadFile(context.Background(), root, "guide.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Sections) != 3 {
		t.Fatalf("sections = %#v", document.Sections)
	}
	if document.Sections[1].Heading != "First" || document.Sections[2].Heading != "Second" {
		t.Fatalf("headings = %#v", document.Sections)
	}
	if !strings.Contains(document.Sections[1].Content, "# not a heading") {
		t.Fatal("fenced Markdown content was lost")
	}
	if document.Sections[2].StartLine != 8 {
		t.Fatalf("second heading start line = %d, want 8", document.Sections[2].StartLine)
	}
}

func TestHTMLAdapterExtractsVisibleTextAndSections(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "page.html"), `<!doctype html><html><head><title>Help &amp; Support</title><style>.bad{}</style></head><body><h1>Wi-Fi</h1><p>Restart &amp; retry.</p><script>alert("hidden")</script><h2>Next</h2><p>Verify.</p></body></html>`)
	document, err := NewLoader().LoadFile(context.Background(), root, "page.html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(document.Content, "alert") || strings.Contains(document.Content, ".bad") {
		t.Fatalf("non-visible HTML leaked into content: %q", document.Content)
	}
	if !strings.Contains(document.Content, "Restart & retry.") || document.Metadata["html_title"] != "Help & Support" {
		t.Fatalf("HTML document = %#v", document)
	}
	if len(document.Sections) < 2 || document.Sections[len(document.Sections)-1].Heading != "Next" {
		t.Fatalf("HTML sections = %#v", document.Sections)
	}
}

func TestJSONAdapterNormalizesKeysAndRejectsTrailingJSON(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "value.json"), `{"z":1,"a":2}`)
	document, err := NewLoader().LoadFile(context.Background(), root, "value.json")
	if err != nil {
		t.Fatal(err)
	}
	if document.Content != "{\n  \"a\": 2,\n  \"z\": 1\n}\n" {
		t.Fatalf("normalized JSON = %q", document.Content)
	}
	mustWrite(t, filepath.Join(root, "bad.json"), `{} {}`)
	if _, err := NewLoader().LoadFile(context.Background(), root, "bad.json"); err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("trailing JSON error = %v", err)
	}
}

func TestLoadFileRejectsTraversalAndEscapingSymlink(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "input")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside.txt")
	mustWrite(t, outside, "secret")
	loader := NewLoader()
	if _, err := loader.LoadFile(context.Background(), root, "../outside.txt"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("traversal error = %v", err)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(root, "outside.txt")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatal(err)
		}
		if _, err := loader.LoadDirectory(context.Background(), root); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("escaping symlink error = %v", err)
		}
	}
}

func TestPDFAdapterReportsMissingPdftotext(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "manual.pdf"), "%PDF-1.4\n")
	loader := NewLoader()
	loader.PDFCommand = "okf-pdftotext-command-that-does-not-exist"
	_, err := loader.LoadFile(context.Background(), root, "manual.pdf")
	if err == nil || !strings.Contains(err.Error(), "requires pdftotext") {
		t.Fatalf("missing pdftotext error = %v", err)
	}
}

func TestStableSourceIDDifferentiatesPaths(t *testing.T) {
	first := StableSourceID("a/guide.md")
	if first != StableSourceID(filepath.Join("a", "guide.md")) {
		t.Fatal("platform path spelling changed stable ID")
	}
	if first == StableSourceID("b/guide.md") {
		t.Fatal("different source paths have identical stable IDs")
	}
}

func mustWrite(t *testing.T, filename, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
