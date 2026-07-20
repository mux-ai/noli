package okf

import (
	"reflect"
	"strings"
	"testing"

	"noli/pkg/graph"
)

func TestStoreSearchIntegerScores(t *testing.T) {
	store, err := Load(writeStoreFixture(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	results := store.Search("Complete Task", SearchOptions{})
	if len(results) == 0 || results[0].ID != "rules/complete-task" {
		t.Fatalf("Search() = %#v", results)
	}
	if results[0].Score != 11 {
		t.Fatalf("seed score = %d, want 11", results[0].Score)
	}
	for _, result := range results {
		if strings.HasSuffix(result.ID, "/index") || result.ID == "index" || result.ID == "log" {
			t.Fatalf("navigation or log leaked into search: %#v", result)
		}
	}
	empty := store.Search("zzz-none", SearchOptions{})
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty Search() = %#v", empty)
	}
}

func TestStoreRetrieveEndToEnd(t *testing.T) {
	store, err := Load(writeStoreFixture(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	result, err := store.Retrieve("Complete Task", RetrieveOptions{})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	ids := make([]string, len(result.Sources))
	for i, source := range result.Sources {
		ids[i] = source.ID
	}
	want := []string{"rules/complete-task", "concepts/task-status", "concepts/todo-item"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("sources = %#v, want %#v", ids, want)
	}
	if !result.Sources[0].Seed || result.Sources[2].Seed {
		t.Fatalf("seed flags = %#v", result.Sources)
	}
	if result.Sources[2].Relationship != "applies-to" || result.Sources[2].Predecessor != "rules/complete-task" {
		t.Fatalf("graph source = %#v", result.Sources[2])
	}
	if !strings.Contains(result.Context, "## Source: rules/complete-task (Business Rule, seed, score 11)") {
		t.Fatalf("context = %q", result.Context)
	}
	if strings.Contains(result.Context, "index") {
		t.Fatalf("navigation leaked into context: %q", result.Context)
	}

	again, err := store.Retrieve("Complete Task", RetrieveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, again) {
		t.Fatal("Store.Retrieve is not deterministic")
	}
}

func TestStoreGraphView(t *testing.T) {
	store, err := Load(writeStoreFixture(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	view, err := store.GraphView("rules/complete-task", GraphOptions{})
	if err != nil {
		t.Fatalf("GraphView() error = %v", err)
	}
	if view.Direction != "both" || view.MaxHops != 1 {
		t.Fatalf("defaults = %#v", view)
	}
	gotNodes := make([]string, len(view.Nodes))
	for i, node := range view.Nodes {
		gotNodes[i] = node.ID
	}
	wantNodes := []string{"rules/complete-task", "concepts/todo-item", "rules/index"}
	if !reflect.DeepEqual(gotNodes, wantNodes) {
		t.Fatalf("nodes = %#v, want %#v", gotNodes, wantNodes)
	}
	wantEdges := []graph.Edge{
		{From: "rules/complete-task", To: "concepts/todo-item", Predicate: "applies-to"},
		{From: "rules/index", To: "rules/complete-task", Predicate: "links-to"},
	}
	if !reflect.DeepEqual(view.Edges, wantEdges) {
		t.Fatalf("edges = %#v, want %#v", view.Edges, wantEdges)
	}
	if view.Nodes[1].Relationship != "applies-to" || view.Nodes[1].Predecessor != "rules/complete-task" {
		t.Fatalf("node record = %#v", view.Nodes[1])
	}

	if _, err := store.GraphView("missing", GraphOptions{}); err == nil {
		t.Fatal("GraphView(missing) succeeded")
	}
	if _, err := store.GraphView("rules/complete-task", GraphOptions{Direction: "sideways"}); err == nil {
		t.Fatal("invalid direction accepted")
	}
	outgoing, err := store.GraphView("rules/complete-task", GraphOptions{Direction: graph.DirectionOutgoing})
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range outgoing.Nodes {
		if node.ID == "rules/index" {
			t.Fatal("outgoing view followed an incoming edge")
		}
	}
}
