package protocol

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// assertMatchesFixture marshals the response and compares it against the
// golden fixture after JSON normalization.
func assertMatchesFixture(t *testing.T, fixture string, response Response) {
	t.Helper()
	var buffer bytes.Buffer
	if err := Write(&buffer, response); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	raw := buffer.Bytes()
	if !bytes.HasSuffix(raw, []byte("\n")) || bytes.Count(raw, []byte("\n")) != 1 {
		t.Fatalf("output must be one line with one trailing newline: %q", raw)
	}
	golden, err := os.ReadFile(filepath.Join("testdata", "fixtures", fixture))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var got, want any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if err := json.Unmarshal(golden, &want); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response does not match fixture %s\ngot:  %s\nwant: %s", fixture, raw, golden)
	}
}

func TestStatusFixture(t *testing.T) {
	assertMatchesFixture(t, "success/status.json", Success("status", StatusData{
		Root:          "examples/todo-app/knowledge",
		BundleID:      "sha256:2b7e151628aed2a6abf7158809cf4f3c762e7160f38b4da56a784d9045190cfe",
		DocumentCount: 16,
		LinkCount:     24,
		Types: []TypeCount{
			{Type: "Application Component", Count: 2},
			{Type: "Architecture Decision", Count: 1},
			{Type: "Business Rule", Count: 3},
			{Type: "Domain Entity", Count: 3},
			{Type: "Workflow", Count: 2},
		},
	}))
}

func TestListFixture(t *testing.T) {
	assertMatchesFixture(t, "success/list.json", Success("list", ListData{
		Count: 2,
		Documents: []DocumentSummary{
			{
				ID: "concepts/todo-item", Type: "Domain Entity", Title: "Todo Item",
				Description: "The core task entity tracked by the Todo application.",
				Tags:        []string{"domain", "todo"},
			},
			{
				ID: "rules/complete-task", Type: "Business Rule", Title: "Complete Task",
				Description: "Rules that govern marking a task as completed.",
				Tags:        []string{"rule", "completion"},
			},
		},
	}))
}

func TestSearchFixture(t *testing.T) {
	assertMatchesFixture(t, "success/search.json", Success("search", SearchData{
		Query: "task completion validation",
		Count: 2,
		Results: []SearchResultItem{
			{
				ID: "rules/complete-task", Type: "Business Rule", Title: "Complete Task",
				Description: "Rules that govern marking a task as completed.", Score: 11,
			},
			{
				ID: "concepts/task-status", Type: "Domain Entity", Title: "Task Status",
				Description: "Lifecycle states of a task.", Score: 6,
			},
		},
	}))
}

func TestRetrieveFixture(t *testing.T) {
	assertMatchesFixture(t, "success/retrieve.json", Success("retrieve", RetrieveData{
		Query: "Implement the CompleteTodo use case",
		Context: "# Context for: Implement the CompleteTodo use case\n\n" +
			"## Source: rules/complete-task (Business Rule, seed, score 11)\n\n" +
			"A task may only be completed when…\n\n" +
			"## Source: concepts/todo-item (Domain Entity, distance 1 via rules/complete-task, applies-to)\n\n" +
			"The core task entity…\n",
		Sources: []RetrieveSource{
			{
				ID: "rules/complete-task", Type: "Business Rule", Title: "Complete Task",
				Seed: true, Score: 11, Distance: 0, Predecessor: "", Relationship: "", Truncated: false,
			},
			{
				ID: "concepts/todo-item", Type: "Domain Entity", Title: "Todo Item",
				Seed: false, Score: 0, Distance: 1, Predecessor: "rules/complete-task",
				Relationship: "applies-to", Truncated: false,
			},
		},
		Statistics: RetrieveStatistics{
			SeedCount: 1, GraphCount: 1, DocumentCount: 2,
			CharacterCount: 1830, MaxCharacters: 14000, Truncated: false,
		},
	}))
}

func TestGetFixture(t *testing.T) {
	assertMatchesFixture(t, "success/get.json", Success("get", GetData{
		Document: DocumentDetail{
			ID: "rules/complete-task", Type: "Business Rule", Title: "Complete Task",
			Description: "Rules that govern marking a task as completed.",
			Tags:        []string{"rule", "completion"},
			Metadata:    map[string]any{"severity": "must"},
			Links: []DocumentLink{
				{Target: "concepts/task-status", Predicate: "links-to"},
				{Target: "concepts/todo-item", Predicate: "applies-to"},
			},
			Body: "## Statement\n\nA task may only be completed when…\n",
		},
	}))
}

func TestGraphFixture(t *testing.T) {
	assertMatchesFixture(t, "success/graph.json", Success("graph", GraphData{
		ID: "rules/complete-task", Direction: "both", MaxHops: 1,
		Nodes: []GraphNodeData{
			{ID: "rules/complete-task", Distance: 0, Predecessor: "", Relationship: ""},
			{ID: "concepts/task-status", Distance: 1, Predecessor: "rules/complete-task", Relationship: "links-to"},
			{ID: "concepts/todo-item", Distance: 1, Predecessor: "rules/complete-task", Relationship: "applies-to"},
		},
		Edges: []GraphEdgeData{
			{From: "rules/complete-task", To: "concepts/task-status", Predicate: "links-to"},
			{From: "rules/complete-task", To: "concepts/todo-item", Predicate: "applies-to"},
		},
	}))
}

func TestValidateFixtures(t *testing.T) {
	// OKF v0.1 section 9: broken links and missing indexes are warnings in
	// standard mode; only a missing type fails the bundle.
	assertMatchesFixture(t, "success/validate-standard.json", Success("validate", ValidateData{
		Mode: "standard", Valid: false,
		Errors: []ValidationProblemData{
			{Code: "MISSING_TYPE", Document: "concepts/untyped", Message: "concept document frontmatter requires a non-empty type"},
		},
		Warnings: []ValidationProblemData{
			{Code: "BROKEN_LINK", Document: "rules/complete-task", Message: "link target \"concepts/missing\" does not exist"},
			{Code: "MISSING_INDEX", Document: "workflows/index", Message: "concept directory has no index.md"},
		},
	}))
	assertMatchesFixture(t, "success/validate-project.json", Success("validate", ValidateData{
		Mode: "project", Valid: true,
		Errors:   []ValidationProblemData{},
		Warnings: []ValidationProblemData{},
	}))
}

func TestGenerateFixtures(t *testing.T) {
	assertMatchesFixture(t, "success/generate-dry-run.json", Success("generate", GenerateData{
		Mode:        "dry-run",
		PreviewRoot: ".noli/preview",
		Added:       []string{"concepts/task-priority"},
		Changed:     []string{"concepts/todo-item"},
		Removed:     []string{},
		Unchanged:   []string{"concepts/task-status", "rules/complete-task"},
	}))
	assertMatchesFixture(t, "success/generate-apply.json", Success("generate", GenerateData{
		Mode:        "apply",
		PreviewRoot: "",
		Added:       []string{"concepts/task-priority"},
		Changed:     []string{"concepts/todo-item"},
		Removed:     []string{},
		Unchanged:   []string{"concepts/task-status", "rules/complete-task"},
	}))
}

func TestPrepareFixture(t *testing.T) {
	assertMatchesFixture(t, "success/prepare-agent-context.json", Success("prepare-agent-context", PrepareData{
		Output:      "/tmp/noli-agent-context-verification",
		BundleID:    "sha256:2b7e151628aed2a6abf7158809cf4f3c762e7160f38b4da56a784d9045190cfe",
		GeneratedAt: "2026-07-20T00:00:00Z",
		Manifest:    "manifest.json",
		Queries: []PrepareQueryData{
			{
				Name:      "complete-todo",
				File:      "complete-todo.md",
				Checksum:  "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
				Sources:   []string{"rules/complete-task", "concepts/todo-item"},
				Truncated: false,
			},
		},
	}))
}

func TestErrorFixtures(t *testing.T) {
	cases := []struct {
		fixture string
		command string
		code    string
		message string
		details map[string]string
	}{
		{"invalid_argument.json", "search", CodeInvalidArgument,
			"flag --limit must be a non-negative integer", map[string]string{"flag": "--limit"}},
		{"knowledge_not_found.json", "status", CodeKnowledgeNotFound,
			"knowledge root \"./missing/knowledge\" does not exist or is not a directory", map[string]string{"root": "./missing/knowledge"}},
		{"document_not_found.json", "get", CodeDocumentNotFound,
			"document \"rules/missing\" was not found", map[string]string{"id": "rules/missing"}},
		{"unsafe_path.json", "prepare-agent-context", CodeUnsafePath,
			"output path escapes the allowed root after symlink resolution", map[string]string{"path": "../outside"}},
		{"parse_error.json", "list", CodeParseError,
			"concepts/todo-item.md: frontmatter block is not terminated", map[string]string{"document": "concepts/todo-item"}},
		{"invalid_frontmatter.json", "list", CodeInvalidFrontmatter,
			"concepts/todo-item.md: frontmatter is not a YAML mapping", map[string]string{"document": "concepts/todo-item"}},
		{"validation_failed.json", "generate", CodeValidationFailed,
			"generated bundle failed validation; active knowledge was left unchanged", map[string]string{"errors": "2"}},
		{"generation_failed.json", "generate", CodeGenerationFailed,
			"concept file .noli/concepts.yaml: relationship target \"concepts/missing\" cannot be resolved", map[string]string{"source": ".noli/concepts.yaml"}},
		{"context_limit_too_small.json", "retrieve", CodeContextLimitTooSmall,
			"--max-characters 10 cannot fit a single source header", map[string]string{"max_characters": "10"}},
		{"internal_error.json", "retrieve", CodeInternalError,
			"an unexpected internal error occurred", nil},
	}
	for _, testCase := range cases {
		assertMatchesFixture(t, filepath.Join("error", testCase.fixture),
			Failure(testCase.command, testCase.code, testCase.message, testCase.details))
	}
}

func TestExitCodeMapping(t *testing.T) {
	expected := map[string]int{
		CodeInvalidArgument:      2,
		CodeContextLimitTooSmall: 2,
		CodeKnowledgeNotFound:    3,
		CodeDocumentNotFound:     3,
		CodeParseError:           3,
		CodeInvalidFrontmatter:   3,
		CodeValidationFailed:     4,
		CodeGenerationFailed:     5,
		CodeUnsafePath:           6,
		CodeInternalError:        7,
	}
	for code, exit := range expected {
		if got := ExitCodeFor(code); got != exit {
			t.Fatalf("ExitCodeFor(%s) = %d, want %d", code, got, exit)
		}
	}
	if ExitCodeFor("SOMETHING_NEW") != ExitInternal {
		t.Fatal("unknown codes must map to the internal exit")
	}
}

func TestWriteHasNoHTMLEscapingOrExtraOutput(t *testing.T) {
	var buffer bytes.Buffer
	if err := Write(&buffer, Success("get", map[string]string{"body": "<a> & </a>"})); err != nil {
		t.Fatal(err)
	}
	output := buffer.String()
	if strings.Contains(output, "\\u003c") {
		t.Fatalf("HTML escaping applied: %q", output)
	}
	if !strings.HasSuffix(output, "}\n") || strings.Count(output, "\n") != 1 {
		t.Fatalf("output = %q", output)
	}
}
