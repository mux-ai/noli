package graph

import (
	"reflect"
	"testing"
)

func visitIDs(visits []Visit) []string {
	ids := make([]string, len(visits))
	for i, visit := range visits {
		ids[i] = visit.ID
	}
	return ids
}

func TestExpandPreservesCallerSeedOrder(t *testing.T) {
	g := New([]string{"z", "a", "m"}, nil)
	visits := g.Expand([]string{"z", "a", "z", " ", "missing", "m"}, ExpandOptions{})
	if got, want := visitIDs(visits), []string{"z", "a", "m"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("seed order = %#v, want %#v", got, want)
	}
	for i, visit := range visits {
		if visit.SeedRank != i || visit.Distance != 0 || visit.Predecessor != "" || visit.Relationship != "" {
			t.Fatalf("seed visit %d = %#v", i, visit)
		}
	}
}

func TestExpandCycleProtectionDeduplicationAndLimit(t *testing.T) {
	g := New(nil, []Edge{
		{From: "a", To: "b", Predicate: "next"},
		{From: "a", To: "c", Predicate: "next"},
		{From: "b", To: "a", Predicate: "next"},
		{From: "b", To: "d", Predicate: "next"},
		{From: "c", To: "d", Predicate: "next"},
	})
	visits := g.Expand([]string{"a", "a"}, ExpandOptions{MaxHops: 10})
	if got, want := visitIDs(visits), []string{"a", "b", "c", "d"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand() = %#v, want %#v", got, want)
	}
	limited := g.Expand([]string{"a"}, ExpandOptions{MaxHops: 10, MaxDocuments: 2})
	if got, want := visitIDs(limited), []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("limited Expand() = %#v, want %#v", got, want)
	}
	seedsOnly := g.Expand([]string{"a"}, ExpandOptions{MaxHops: 0, MaxDocuments: 10})
	if got, want := visitIDs(seedsOnly), []string{"a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MaxHops 0 Expand() = %#v, want %#v", got, want)
	}
}

func TestExpandRecordsDistancePredecessorAndRelationship(t *testing.T) {
	g := New(nil, []Edge{
		{From: "rule", To: "entity", Predicate: "applies-to"},
		{From: "entity", To: "component", Predicate: "uses"},
	})
	visits := g.Expand([]string{"rule"}, ExpandOptions{MaxHops: 2, MaxDocuments: 10})
	want := []Visit{
		{ID: "rule", Distance: 0, Predecessor: "", Relationship: "", SeedRank: 0},
		{ID: "entity", Distance: 1, Predecessor: "rule", Relationship: "applies-to", SeedRank: 0},
		{ID: "component", Distance: 2, Predecessor: "entity", Relationship: "uses", SeedRank: 0},
	}
	if !reflect.DeepEqual(visits, want) {
		t.Fatalf("Expand() = %#v, want %#v", visits, want)
	}
}

func TestExpandWithinDistanceOrdering(t *testing.T) {
	// Seed order is v then u; u links alphabetically earlier nodes. Ordering
	// within distance 1 must be seed rank first, then relationship, then ID.
	g := New(nil, []Edge{
		{From: "v", To: "z", Predicate: "uses"},
		{From: "v", To: "y", Predicate: "uses"},
		{From: "v", To: "x", Predicate: "applies-to"},
		{From: "u", To: "b", Predicate: "aaa"},
	})
	visits := g.Expand([]string{"v", "u"}, ExpandOptions{MaxHops: 1, MaxDocuments: 10})
	want := []string{"v", "u", "x", "y", "z", "b"}
	if got := visitIDs(visits); !reflect.DeepEqual(got, want) {
		t.Fatalf("ordering = %#v, want %#v", got, want)
	}
	if visits[2].Relationship != "applies-to" || visits[2].SeedRank != 0 {
		t.Fatalf("visit x = %#v", visits[2])
	}
	if visits[5].SeedRank != 1 {
		t.Fatalf("visit b = %#v", visits[5])
	}
}

func TestExpandFirstDiscoveryWins(t *testing.T) {
	// d is reachable from both seeds; the earlier seed rank must claim it.
	g := New(nil, []Edge{
		{From: "s1", To: "d", Predicate: "zzz"},
		{From: "s2", To: "d", Predicate: "aaa"},
	})
	visits := g.Expand([]string{"s1", "s2"}, ExpandOptions{MaxHops: 1, MaxDocuments: 10})
	if len(visits) != 3 {
		t.Fatalf("Expand() = %#v", visits)
	}
	d := visits[2]
	if d.ID != "d" || d.SeedRank != 0 || d.Predecessor != "s1" || d.Relationship != "zzz" {
		t.Fatalf("visit d = %#v", d)
	}
}

func TestExpandDirections(t *testing.T) {
	g := New(nil, []Edge{
		{From: "a", To: "b", Predicate: "uses"},
		{From: "c", To: "a", Predicate: "owns"},
	})
	outgoing := g.Expand([]string{"a"}, ExpandOptions{MaxHops: 1, Direction: DirectionOutgoing})
	if got, want := visitIDs(outgoing), []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("outgoing = %#v, want %#v", got, want)
	}
	incoming := g.Expand([]string{"a"}, ExpandOptions{MaxHops: 1, Direction: DirectionIncoming})
	if got, want := visitIDs(incoming), []string{"a", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("incoming = %#v, want %#v", got, want)
	}
	if incoming[1].Predecessor != "a" || incoming[1].Relationship != "owns" {
		t.Fatalf("incoming visit = %#v", incoming[1])
	}
	// Within a distance the order is relationship, then ID: "owns" < "uses".
	both := g.Expand([]string{"a"}, ExpandOptions{MaxHops: 1, Direction: DirectionBoth})
	if got, want := visitIDs(both), []string{"a", "c", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("both = %#v, want %#v", got, want)
	}
}

func TestExpandPredicateFilter(t *testing.T) {
	g := New(nil, []Edge{
		{From: "a", To: "b", Predicate: "uses"},
		{From: "a", To: "c", Predicate: "follows"},
		{From: "d", To: "a", Predicate: "owns"},
	})
	visits := g.Expand([]string{"a"}, ExpandOptions{
		MaxHops: 1, Direction: DirectionBoth, Predicates: []string{" OWNS ", "uses"},
	})
	if got, want := visitIDs(visits), []string{"a", "d", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered = %#v, want %#v", got, want)
	}
}

func TestExpandSeedLimitAndEmptyResultIsNonNil(t *testing.T) {
	g := New([]string{"a", "b", "c"}, nil)
	visits := g.Expand([]string{"a", "b", "c"}, ExpandOptions{MaxDocuments: 2})
	if got, want := visitIDs(visits), []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("seed limit = %#v, want %#v", got, want)
	}
	empty := g.Expand([]string{"missing"}, ExpandOptions{MaxHops: 3})
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty Expand() = %#v", empty)
	}
}
