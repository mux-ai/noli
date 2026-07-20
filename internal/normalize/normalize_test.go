package normalize

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func normalizedTestDocument() Document {
	return Document{
		ID: "source-0123456789", Name: " guide.md ", SourceURI: "docs/guide.md", MediaType: "Text/Markdown",
		Content:  "\ufeff# Guide  \r\n\r\n\r\n\r\nUse it.\t \r\n",
		Sections: []Section{{Heading: " Guide   Overview ", Content: "Use it.  \r\n", StartLine: 1, EndLine: 2}},
		Metadata: map[string]any{"custom": "preserved", "priority": float64(2)},
	}
}

func TestNormalizeDocumentCleansTextAndPreservesMetadata(t *testing.T) {
	document, err := NormalizeDocument(normalizedTestDocument())
	if err != nil {
		t.Fatal(err)
	}
	if document.Name != "guide.md" || document.MediaType != "text/markdown" {
		t.Fatalf("normalized identity = %#v", document)
	}
	if strings.Contains(document.Content, "\r") || strings.Contains(document.Content, "\ufeff") || strings.Contains(document.Content, "  \n") {
		t.Fatalf("content was not cleaned: %q", document.Content)
	}
	if document.Sections[0].Heading != "Guide Overview" {
		t.Fatalf("heading = %q", document.Sections[0].Heading)
	}
	if document.Metadata["custom"] != "preserved" {
		t.Fatalf("metadata = %#v", document.Metadata)
	}
}

func TestNormalizeDocumentsStableOrderAndDuplicateRejection(t *testing.T) {
	first := normalizedTestDocument()
	second := normalizedTestDocument()
	second.ID = "source-0000000000"
	second.SourceURI = "docs/other.md"
	documents, err := NormalizeDocuments([]Document{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if documents[0].ID != second.ID {
		t.Fatalf("documents not sorted: %#v", documents)
	}
	second.ID = first.ID
	if _, err := NormalizeDocuments([]Document{first, second}); err == nil || !strings.Contains(err.Error(), "duplicate source ID") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestChunkerHonorsRuneLimitAndIsDeterministic(t *testing.T) {
	document := normalizedTestDocument()
	document.Content = "αβγδε ζηθικ λμνξο πρστυ φχψω"
	document.Sections = []Section{{Heading: "Greek", Content: document.Content, StartLine: 1, EndLine: 1}}
	chunker := Chunker{MaximumCharacters: 12}
	first, err := chunker.Chunk(document)
	if err != nil {
		t.Fatal(err)
	}
	second, err := chunker.Chunk(document)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("chunking was not deterministic")
	}
	if len(first) < 2 {
		t.Fatalf("chunks = %#v, want multiple", first)
	}
	for _, chunk := range first {
		if utf8.RuneCountInString(chunk.Content) > 12 {
			t.Errorf("chunk %s has %d runes: %q", chunk.ID, utf8.RuneCountInString(chunk.Content), chunk.Content)
		}
	}
}

func TestWriteAndLoadDocumentsDeterministically(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "staging", "normalized")
	document := normalizedTestDocument()
	if err := WriteDocuments(directory, []Document{document}); err != nil {
		t.Fatal(err)
	}
	manifestFirst, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadDocuments(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Metadata["custom"] != "preserved" {
		t.Fatalf("loaded documents = %#v", loaded)
	}
	if err := os.WriteFile(filepath.Join(directory, "stale.json"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteDocuments(directory, []Document{document}); err != nil {
		t.Fatal(err)
	}
	manifestSecond, _ := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if string(manifestFirst) != string(manifestSecond) {
		t.Fatal("manifest changed between identical writes")
	}
	if _, err := os.Stat(filepath.Join(directory, "stale.json")); !os.IsNotExist(err) {
		t.Fatalf("stale normalized file survived replacement: %v", err)
	}
}

func TestNormalizeRejectsUnsafeIDAndNULText(t *testing.T) {
	document := normalizedTestDocument()
	document.ID = "../escape"
	if _, err := NormalizeDocument(document); err == nil {
		t.Fatal("NormalizeDocument accepted unsafe ID")
	}
	document = normalizedTestDocument()
	document.Content = "bad\x00text"
	if _, err := NormalizeDocument(document); err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("NUL error = %v", err)
	}
}
