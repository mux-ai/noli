package graph

import (
	"reflect"
	"testing"
)

func TestNewNormalizesAndDeduplicatesEdges(t *testing.T) {
	g := New([]string{"a", " ", "b"}, []Edge{
		{From: "a", To: "b", Predicate: ""},
		{From: "a", To: "b", Predicate: "links-to"},
		{From: " a ", To: " b ", Predicate: " links-to "},
		{From: "", To: "b", Predicate: "uses"},
		{From: "a", To: "", Predicate: "uses"},
		{From: "b", To: "c", Predicate: "depends-on"},
	})
	if got, want := g.EdgeCount(), 2; got != want {
		t.Fatalf("EdgeCount() = %d, want %d", got, want)
	}
	if !g.HasNode("c") {
		t.Fatal("edge endpoint was not added as a node")
	}
	if got, want := g.Nodes(), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Nodes() = %#v, want %#v", got, want)
	}
	outgoing := g.Outgoing("a")
	if len(outgoing) != 1 || outgoing[0].Predicate != PredicateLinksTo {
		t.Fatalf("Outgoing(a) = %#v", outgoing)
	}
}

func TestAccessorsReturnCopies(t *testing.T) {
	g := New(nil, []Edge{{From: "a", To: "b", Predicate: "uses"}})
	edges := g.Outgoing("a")
	edges[0].To = "changed"
	if g.Outgoing("a")[0].To != "b" {
		t.Fatal("Outgoing exposed internal storage")
	}
	nodes := g.Nodes()
	nodes[0] = "changed"
	if g.Nodes()[0] != "a" {
		t.Fatal("Nodes exposed internal storage")
	}
}

func TestNeighborsDirections(t *testing.T) {
	g := New(nil, []Edge{
		{From: "a", To: "b", Predicate: "uses"},
		{From: "c", To: "a", Predicate: "owns"},
	})
	if got := g.Neighbors("a", DirectionOutgoing); len(got) != 1 || got[0].To != "b" {
		t.Fatalf("Neighbors(outgoing) = %#v", got)
	}
	if got := g.Neighbors("a", DirectionIncoming); len(got) != 1 || got[0].From != "c" {
		t.Fatalf("Neighbors(incoming) = %#v", got)
	}
	both := g.Neighbors("a", DirectionBoth)
	if len(both) != 2 {
		t.Fatalf("Neighbors(both) = %#v", both)
	}
	want := []Edge{
		{From: "a", To: "b", Predicate: "uses"},
		{From: "c", To: "a", Predicate: "owns"},
	}
	if !reflect.DeepEqual(both, want) {
		t.Fatalf("Neighbors(both) = %#v, want %#v", both, want)
	}
	if got := g.Neighbors("a", ""); len(got) != 1 || got[0].To != "b" {
		t.Fatalf("Neighbors(default) = %#v", got)
	}
}

func TestNilGraphIsSafe(t *testing.T) {
	var g *Graph
	if g.HasNode("a") || g.NodeCount() != 0 || g.EdgeCount() != 0 {
		t.Fatal("nil graph reported content")
	}
	if got := g.Nodes(); got == nil || len(got) != 0 {
		t.Fatalf("nil Nodes() = %#v", got)
	}
	if got := g.Expand([]string{"a"}, ExpandOptions{}); got == nil || len(got) != 0 {
		t.Fatalf("nil Expand() = %#v", got)
	}
	if got := g.Neighbors("a", DirectionBoth); got == nil || len(got) != 0 {
		t.Fatalf("nil Neighbors() = %#v", got)
	}
}
