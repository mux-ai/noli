package retrieval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"noli/pkg/internal/targetlock"
)

const validQueries = `version: 1
queries:
  - name: complete-todo
    query: Implement the CompleteTodo use case
    types: [Business Rule, Domain Entity]
    search_limit: 5
    max_hops: 1
    max_documents: 8
    max_characters: 14000
    direction: both
  - name: create-task
    query: Create a task
`

func fakeRetrieve(query string, options Options) (Result, error) {
	return Result{
		Query:   query,
		Context: "# Context for: " + query + "\n\ncontent for " + query + "\n",
		Sources: []Source{{ID: "rules/complete-task", Seed: true, Score: 11}},
		Statistics: Statistics{
			SeedCount: 1, DocumentCount: 1,
			MaxCharacters: options.MaxCharacters,
		},
	}, nil
}

func TestParseQueriesValidAndDefaults(t *testing.T) {
	queries, err := ParseQueries([]byte(validQueries))
	if err != nil {
		t.Fatalf("ParseQueries() error = %v", err)
	}
	if len(queries) != 2 {
		t.Fatalf("queries = %#v", queries)
	}
	first := queries[0]
	if first.Name != "complete-todo" || first.Options.MaxDocuments != 8 || first.Options.MaxCharacters != 14000 {
		t.Fatalf("first = %#v", first)
	}
	second := queries[1]
	if second.Options.SearchLimit != DefaultSearchLimit || second.Options.MaxCharacters != DefaultMaxCharacters {
		t.Fatalf("defaults not applied: %#v", second.Options)
	}
}

func TestParseQueriesRejectsBadInput(t *testing.T) {
	cases := []struct {
		name    string
		content string
		message string
	}{
		{"unknown field", validQueries + "surprise: true\n", "field surprise not found"},
		{"bad version", strings.Replace(validQueries, "version: 1", "version: 9", 1), "version must be"},
		{"no queries", "version: 1\nqueries: []\n", "at least one"},
		{"uppercase name", strings.Replace(validQueries, "name: complete-todo", "name: CompleteTodo", 1), "must match"},
		{"path name", strings.Replace(validQueries, "name: complete-todo", "name: ../escape", 1), "must match"},
		{"duplicate name", strings.Replace(validQueries, "name: create-task", "name: complete-todo", 1), "duplicate"},
		{"empty query", strings.Replace(validQueries, "query: Create a task", "query: \"\"", 1), "query text is required"},
		{"bad direction", strings.Replace(validQueries, "direction: both", "direction: sideways", 1), "direction"},
		{"negative bound", strings.Replace(validQueries, "max_hops: 1", "max_hops: -1", 1), "negative"},
	}
	for _, testCase := range cases {
		_, err := ParseQueries([]byte(testCase.content))
		if err == nil || !strings.Contains(err.Error(), testCase.message) {
			t.Fatalf("%s: error = %v, want containing %q", testCase.name, err, testCase.message)
		}
	}
}

func TestPrepareWritesFilesAndReproducibleManifest(t *testing.T) {
	queries, err := ParseQueries([]byte(validQueries))
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "prepared")
	clock := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	prepared, err := Prepare(queries, fakeRetrieve, PrepareOptions{
		Output: output, BundleID: "sha256:abc", GeneratedAt: clock,
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.GeneratedAt != "2026-07-20T12:00:00Z" || prepared.Manifest != ManifestFileName {
		t.Fatalf("prepared = %#v", prepared)
	}

	// Every checksum in the manifest must reproduce from the file bytes.
	manifestData, err := os.ReadFile(filepath.Join(output, ManifestFileName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["bundle_id"] != "sha256:abc" || manifest["generated_at"] != "2026-07-20T12:00:00Z" {
		t.Fatalf("manifest = %#v", manifest)
	}
	for _, entry := range manifest["queries"].([]any) {
		query := entry.(map[string]any)
		data, err := os.ReadFile(filepath.Join(output, query["file"].(string)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if query["checksum"] != "sha256:"+hex.EncodeToString(sum[:]) {
			t.Fatalf("checksum mismatch for %v", query["file"])
		}
	}

	// A second run with the same clock reproduces the manifest exactly.
	second, err := Prepare(queries, fakeRetrieve, PrepareOptions{
		Output: output, BundleID: "sha256:abc", GeneratedAt: clock,
	})
	if err != nil {
		t.Fatalf("second Prepare() error = %v", err)
	}
	if !reflect.DeepEqual(prepared, second) {
		t.Fatal("prepared results differ across runs")
	}
	secondManifest, err := os.ReadFile(filepath.Join(output, ManifestFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(manifestData) != string(secondManifest) {
		t.Fatal("manifest bytes differ across identical runs")
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(output), "."+filepath.Base(output)+".preparing-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary paths = %v, error = %v", leftovers, err)
	}
}

func TestPrepareRejectsConcurrentWriter(t *testing.T) {
	queries, err := ParseQueries([]byte(validQueries))
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "prepared")
	lock, err := targetlock.Acquire(output + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()

	_, err = Prepare(queries, fakeRetrieve, PrepareOptions{
		Output: output, BundleID: "sha256:abc", GeneratedAt: time.Unix(0, 0),
	})
	if !errors.Is(err, ErrWriteBusy) {
		t.Fatalf("Prepare() error = %v, want ErrWriteBusy", err)
	}
}

func TestPrepareRefusesUnrelatedOutput(t *testing.T) {
	queries, err := ParseQueries([]byte(validQueries))
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	if err := os.WriteFile(filepath.Join(output, "important.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Prepare(queries, fakeRetrieve, PrepareOptions{Output: output, GeneratedAt: time.Unix(0, 0)})
	if !errors.Is(err, ErrUnsafeOutput) {
		t.Fatalf("error = %v, want ErrUnsafeOutput", err)
	}
	if data, err := os.ReadFile(filepath.Join(output, "important.txt")); err != nil || string(data) != "keep" {
		t.Fatal("unrelated output was modified")
	}
	if _, err := Prepare(queries, fakeRetrieve, PrepareOptions{Output: "bad\x00path", GeneratedAt: time.Unix(0, 0)}); !errors.Is(err, ErrUnsafeOutput) {
		t.Fatalf("NUL output error = %v", err)
	}
}

func TestPreparePropagatesRetrievalFailure(t *testing.T) {
	queries, err := ParseQueries([]byte(validQueries))
	if err != nil {
		t.Fatal(err)
	}
	failing := func(query string, options Options) (Result, error) {
		return Result{}, ErrContextLimitTooSmall
	}
	output := filepath.Join(t.TempDir(), "prepared")
	_, err = Prepare(queries, failing, PrepareOptions{Output: output, GeneratedAt: time.Unix(0, 0)})
	if !errors.Is(err, ErrContextLimitTooSmall) {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatal("failed prepare left output behind")
	}
}

func TestPrepareTruncationFlagInManifest(t *testing.T) {
	queries, err := ParseQueries([]byte(validQueries))
	if err != nil {
		t.Fatal(err)
	}
	truncating := func(query string, options Options) (Result, error) {
		result, _ := fakeRetrieve(query, options)
		result.Statistics.Truncated = true
		return result, nil
	}
	output := filepath.Join(t.TempDir(), "prepared")
	prepared, err := Prepare(queries, truncating, PrepareOptions{Output: output, GeneratedAt: time.Unix(0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range prepared.Queries {
		if !query.Truncated {
			t.Fatalf("truncation flag lost: %#v", query)
		}
	}
}
