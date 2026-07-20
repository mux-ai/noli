package search

import (
	"reflect"
	"testing"
)

func testRecords() []Record {
	return []Record{
		{ID: "guides/title", Type: "Guide", Title: "Wi-Fi connection"},
		{ID: "guides/body", Type: "Guide", Title: "Other", Body: "Wi-Fi connection"},
		{ID: "products/meta", Type: "Product", Title: "Device",
			Metadata: map[string]any{"supported_protocol": "Wi-Fi connection"}},
		{ID: "index", Type: "Navigation", Title: "Wi-Fi connection", Navigation: true},
		{ID: "log", Type: "Log", Title: "Wi-Fi connection", Log: true},
	}
}

func TestSearchIntegerScoresAndFieldWeights(t *testing.T) {
	results := NewIndex(testRecords()).Search("Wi-Fi connection", Options{Limit: 10})
	want := []Result{
		{ID: "guides/title", Type: "Guide", Title: "Wi-Fi connection", Score: 11},
		{ID: "products/meta", Type: "Product", Title: "Device", Score: 5},
		{ID: "guides/body", Type: "Guide", Title: "Other", Score: 3},
	}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("Search() = %#v, want %#v", results, want)
	}
}

func TestSearchNavigationAndLogInclusion(t *testing.T) {
	index := NewIndex(testRecords())
	withNavigation := index.Search("Wi-Fi", Options{Limit: 10, IncludeNavigation: true})
	foundNavigation := false
	for _, result := range withNavigation {
		if result.ID == "index" {
			foundNavigation = true
		}
		if result.ID == "log" {
			t.Fatal("log document included without IncludeLog")
		}
	}
	if !foundNavigation {
		t.Fatalf("navigation not included: %#v", withNavigation)
	}
	withLog := index.Search("Wi-Fi", Options{Limit: 10, IncludeLog: true})
	foundLog := false
	for _, result := range withLog {
		if result.ID == "log" {
			foundLog = true
		}
	}
	if !foundLog {
		t.Fatalf("log not included: %#v", withLog)
	}
}

func TestSearchTypeFilters(t *testing.T) {
	index := NewIndex(testRecords())
	included := index.Search("Wi-Fi connection", Options{Limit: 10, IncludeTypes: []string{" guide "}})
	if len(included) != 2 {
		t.Fatalf("IncludeTypes = %#v", included)
	}
	for _, result := range included {
		if result.Type != "Guide" {
			t.Fatalf("IncludeTypes leaked %#v", result)
		}
	}
	excluded := index.Search("Wi-Fi connection", Options{Limit: 10, ExcludeTypes: []string{"GUIDE"}})
	if len(excluded) != 1 || excluded[0].ID != "products/meta" {
		t.Fatalf("ExcludeTypes = %#v", excluded)
	}
}

func TestSearchPhraseBonusPerField(t *testing.T) {
	records := []Record{{ID: "doc", Type: "Guide", Title: "alpha beta", Body: "alpha beta"}}
	results := NewIndex(records).Search("alpha beta", Options{Limit: 10})
	// title: 2 tokens * 5 + 1 phrase = 11; body: 2 tokens * 1 + 1 phrase = 3.
	if len(results) != 1 || results[0].Score != 14 {
		t.Fatalf("Search() = %#v", results)
	}
}

func TestSearchDeterministicTieOrderAndLimit(t *testing.T) {
	records := []Record{
		{ID: "z", Title: "same"},
		{ID: "a", Title: "same"},
	}
	index := NewIndex(records)
	results := index.Search("same", Options{Limit: 1})
	if len(results) != 1 || results[0].ID != "a" {
		t.Fatalf("Search() = %#v", results)
	}
	all := index.Search("same", Options{})
	if len(all) != 2 || all[0].ID != "a" || all[1].ID != "z" {
		t.Fatalf("unlimited Search() = %#v", all)
	}
	negative := index.Search("same", Options{Limit: -1})
	if negative == nil || len(negative) != 0 {
		t.Fatalf("negative limit Search() = %#v", negative)
	}
	empty := index.Search("   ", Options{Limit: 10})
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty query Search() = %#v", empty)
	}
}

func TestSearchDoesNotMutateCallerRecords(t *testing.T) {
	records := []Record{{ID: "b", Title: "match"}, {ID: "a", Title: "match"}}
	NewIndex(records)
	if records[0].ID != "b" {
		t.Fatal("NewIndex re-sorted the caller's slice")
	}
}

func TestTokenizeUnicodeAndHyphens(t *testing.T) {
	if got := tokenize("API_v2 Wi-Fi 日本語"); !reflect.DeepEqual(got, []string{"api_v2", "wi-fi", "日本語"}) {
		t.Fatalf("tokenize() = %#v", got)
	}
	if got := tokenize("--- - --"); got != nil {
		t.Fatalf("hyphen-only tokenize() = %#v", got)
	}
	if got := normalizePhrase("  Wi-Fi \t Connection\n"); got != "wi-fi connection" {
		t.Fatalf("normalizePhrase() = %q", got)
	}
}

func TestMetadataTextDeterministicNestedFlattening(t *testing.T) {
	metadata := map[string]any{
		"zeta":  []any{"one", 2},
		"alpha": map[string]any{"inner": "value"},
	}
	first := metadataText(metadata)
	second := metadataText(metadata)
	if first != second {
		t.Fatalf("metadataText not deterministic: %q vs %q", first, second)
	}
	if want := "alpha inner value zeta one 2 "; first != want {
		t.Fatalf("metadataText() = %q, want %q", first, want)
	}
}
