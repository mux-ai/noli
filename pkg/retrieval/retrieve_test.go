package retrieval

import (
	"errors"
	"reflect"
	"testing"
	"unicode/utf8"

	"noli/pkg/graph"
	"noli/pkg/search"
)

func fixture() ([]Record, *graph.Graph, *search.Index) {
	records := []Record{
		{ID: "rules/complete-task", Type: "Business Rule", Title: "Complete Task", Content: "Rule statement."},
		{ID: "concepts/todo-item", Type: "Domain Entity", Title: "Todo Item", Content: "Item definition."},
		{ID: "concepts/task-status", Type: "Domain Entity", Title: "Status", Content: "Status states."},
		{ID: "components/todo-service", Type: "Application Component", Title: "Todo Service", Content: "Service behavior."},
		{ID: "index", Type: "Navigation", Title: "Home", Content: "Navigation.", Navigation: true},
		{ID: "log", Type: "Bundle Log", Title: "Log", Content: "Log entries.", Log: true},
		{ID: "notes/other", Type: "Note", Title: "Other", Content: "Other note."},
	}
	nodes := make([]string, len(records))
	searchRecords := make([]search.Record, len(records))
	for i, record := range records {
		nodes[i] = record.ID
		searchRecords[i] = search.Record{
			ID: record.ID, Type: record.Type, Title: record.Title,
			Description: record.Description, Body: record.Content,
			Navigation: record.Navigation, Log: record.Log,
		}
	}
	g := graph.New(nodes, []graph.Edge{
		{From: "rules/complete-task", To: "concepts/todo-item", Predicate: "applies-to"},
		{From: "rules/complete-task", To: "concepts/task-status", Predicate: "uses"},
		{From: "rules/complete-task", To: "index", Predicate: "links-to"},
		{From: "rules/complete-task", To: "notes/other", Predicate: "links-to"},
		{From: "components/todo-service", To: "rules/complete-task", Predicate: "enforced-by"},
	})
	return records, g, search.NewIndex(searchRecords)
}

func sourceIDs(sources []Source) []string {
	ids := make([]string, len(sources))
	for i, source := range sources {
		ids[i] = source.ID
	}
	return ids
}

func TestRetrieveOrderingAndNavigationExclusion(t *testing.T) {
	records, g, index := fixture()
	result, err := Retrieve("complete task", records, g, index, Options{})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	// Distance 1 order is relationship, then ID: applies-to, enforced-by,
	// links-to, uses. The navigation index document is excluded.
	want := []string{
		"rules/complete-task",
		"concepts/todo-item",
		"components/todo-service",
		"notes/other",
		"concepts/task-status",
	}
	if got := sourceIDs(result.Sources); !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %#v, want %#v", got, want)
	}
	seed := result.Sources[0]
	if !seed.Seed || seed.Score != 11 || seed.Distance != 0 || seed.Predecessor != "" || seed.Relationship != "" {
		t.Fatalf("seed = %#v", seed)
	}
	item := result.Sources[1]
	if item.Seed || item.Score != 0 || item.Distance != 1 || item.Predecessor != "rules/complete-task" || item.Relationship != "applies-to" {
		t.Fatalf("graph source = %#v", item)
	}
	stats := result.Statistics
	if stats.SeedCount != 1 || stats.GraphCount != 4 || stats.DocumentCount != 5 || stats.Truncated {
		t.Fatalf("statistics = %#v", stats)
	}
	if stats.CharacterCount != utf8.RuneCountInString(result.Context) || stats.MaxCharacters != DefaultMaxCharacters {
		t.Fatalf("character statistics = %#v", stats)
	}
}

func TestRetrieveTypeFiltersApplyToExpandedCandidates(t *testing.T) {
	records, g, index := fixture()
	excluded, err := Retrieve("complete task", records, g, index, Options{ExcludeTypes: []string{"note"}})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	for _, source := range excluded.Sources {
		if source.ID == "notes/other" {
			t.Fatalf("excluded type leaked: %#v", excluded.Sources)
		}
	}
	included, err := Retrieve("complete task", records, g, index, Options{
		IncludeTypes: []string{"Business Rule", "Domain Entity"},
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	want := []string{"rules/complete-task", "concepts/todo-item", "concepts/task-status"}
	if got := sourceIDs(included.Sources); !reflect.DeepEqual(got, want) {
		t.Fatalf("included sources = %#v, want %#v", got, want)
	}
}

func TestRetrieveContextFormatExact(t *testing.T) {
	records, g, index := fixture()
	result, err := Retrieve("complete task", records, g, index, Options{MaxDocuments: 2})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	want := "# Context for: complete task\n" +
		"\n## Source: rules/complete-task (Business Rule, seed, score 11)\n\n" +
		"Rule statement.\n" +
		"\n## Source: concepts/todo-item (Domain Entity, distance 1 via rules/complete-task, applies-to)\n\n" +
		"Item definition.\n"
	if result.Context != want {
		t.Fatalf("context = %q, want %q", result.Context, want)
	}
	if result.Statistics.DocumentCount != 2 || result.Statistics.Truncated {
		t.Fatalf("statistics = %#v", result.Statistics)
	}
}

func TestRetrieveTruncationIsRuneSafe(t *testing.T) {
	records := []Record{{
		ID: "docs/unicode", Type: "Note", Title: "Unicode complete task",
		Content: "日本語のテキストが続きます。もっともっと続きます。",
	}}
	g := graph.New([]string{"docs/unicode"}, nil)
	index := search.NewIndex([]search.Record{{ID: "docs/unicode", Type: "Note", Title: "Unicode complete task"}})
	full, err := Retrieve("complete task", records, g, index, Options{})
	if err != nil {
		t.Fatal(err)
	}
	budget := full.Statistics.CharacterCount - 5
	result, err := Retrieve("complete task", records, g, index, Options{MaxCharacters: budget})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if !utf8.ValidString(result.Context) {
		t.Fatal("context contains invalid UTF-8 after truncation")
	}
	if len(result.Sources) != 1 || !result.Sources[0].Truncated || !result.Statistics.Truncated {
		t.Fatalf("truncation flags = %#v", result)
	}
	if got := utf8.RuneCountInString(result.Context); got > budget {
		t.Fatalf("context runes = %d, budget %d", got, budget)
	}
}

func TestRetrieveContextLimitTooSmall(t *testing.T) {
	records, g, index := fixture()
	_, err := Retrieve("complete task", records, g, index, Options{MaxCharacters: 10})
	if !errors.Is(err, ErrContextLimitTooSmall) {
		t.Fatalf("error = %v, want ErrContextLimitTooSmall", err)
	}
}

func TestRetrieveDropsSectionsBeyondBudgetDeterministically(t *testing.T) {
	records, g, index := fixture()
	two, err := Retrieve("complete task", records, g, index, Options{MaxDocuments: 2})
	if err != nil {
		t.Fatal(err)
	}
	// Budget exactly fits the first two sections; the remaining selected
	// documents are dropped and the statistics say so.
	budget := two.Statistics.CharacterCount
	result, err := Retrieve("complete task", records, g, index, Options{MaxCharacters: budget})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	want := []string{"rules/complete-task", "concepts/todo-item"}
	if got := sourceIDs(result.Sources); !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %#v, want %#v", got, want)
	}
	if !result.Statistics.Truncated {
		t.Fatal("dropped sections did not set Truncated")
	}
	if result.Sources[0].Truncated || result.Sources[1].Truncated {
		t.Fatal("complete sections were marked truncated")
	}
}

func TestRetrieveMaxHopsAndMaxDocuments(t *testing.T) {
	records, g, index := fixture()
	seedsOnly, err := Retrieve("complete task", records, g, index, Options{MaxDocuments: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := sourceIDs(seedsOnly.Sources); !reflect.DeepEqual(got, []string{"rules/complete-task"}) {
		t.Fatalf("MaxDocuments=1 sources = %#v", got)
	}
	outgoing, err := Retrieve("complete task", records, g, index, Options{Direction: graph.DirectionOutgoing})
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range outgoing.Sources {
		if source.ID == "components/todo-service" {
			t.Fatal("outgoing direction followed an incoming edge")
		}
	}
}

func TestRetrieveNoSeedsReturnsEmptyResult(t *testing.T) {
	records, g, index := fixture()
	result, err := Retrieve("zzz-nothing-matches", records, g, index, Options{})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result.Context != "" || len(result.Sources) != 0 || result.Sources == nil {
		t.Fatalf("empty result = %#v", result)
	}
	if result.Statistics.DocumentCount != 0 || result.Statistics.MaxCharacters != DefaultMaxCharacters {
		t.Fatalf("statistics = %#v", result.Statistics)
	}
}

func TestRetrieveDeterministicAcrossRuns(t *testing.T) {
	records, g, index := fixture()
	first, err := Retrieve("complete task", records, g, index, Options{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Retrieve("complete task", records, g, index, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("two identical retrievals differ")
	}
}

func TestRetrieveRejectsInvalidOptions(t *testing.T) {
	records, g, index := fixture()
	if _, err := Retrieve("q", records, g, index, Options{MaxDocuments: -1}); err == nil {
		t.Fatal("negative bounds accepted")
	}
	if _, err := Retrieve("q", records, g, index, Options{Direction: "sideways"}); err == nil {
		t.Fatal("invalid direction accepted")
	}
}
