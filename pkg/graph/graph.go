// Package graph provides a deterministic, dependency-free directed graph over
// document IDs and typed edges. It never imports OKF document types; callers
// adapt their models into IDs and Edge values.
package graph

import (
	"slices"
	"sort"
	"strings"
)

// Direction selects which edges a traversal follows.
type Direction string

const (
	DirectionOutgoing Direction = "outgoing"
	DirectionIncoming Direction = "incoming"
	DirectionBoth     Direction = "both"
)

// PredicateLinksTo is the predicate assigned to ordinary untyped links.
const PredicateLinksTo = "links-to"

// Edge is a typed, directed connection between two document IDs. An empty
// predicate is normalized to PredicateLinksTo when the edge is added.
type Edge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Predicate string `json:"predicate"`
}

// Graph is an immutable adjacency structure. Build it once with New; all
// accessors return copies, never internal storage.
type Graph struct {
	nodes    map[string]struct{}
	outgoing map[string][]Edge
	incoming map[string][]Edge
}

// New builds a graph from explicit nodes and edges. Edge endpoints are added
// as nodes automatically. Blank node IDs and edges with a blank endpoint are
// ignored. Duplicate edges are stored once.
func New(nodes []string, edges []Edge) *Graph {
	g := &Graph{
		nodes:    make(map[string]struct{}, len(nodes)),
		outgoing: make(map[string][]Edge),
		incoming: make(map[string][]Edge),
	}
	for _, node := range nodes {
		if id := strings.TrimSpace(node); id != "" {
			g.nodes[id] = struct{}{}
		}
	}
	for _, edge := range edges {
		g.addEdge(edge)
	}
	for id := range g.outgoing {
		sortEdges(g.outgoing[id])
	}
	for id := range g.incoming {
		sortEdges(g.incoming[id])
	}
	return g
}

func (g *Graph) addEdge(edge Edge) {
	from := strings.TrimSpace(edge.From)
	to := strings.TrimSpace(edge.To)
	if from == "" || to == "" {
		return
	}
	predicate := strings.TrimSpace(edge.Predicate)
	if predicate == "" {
		predicate = PredicateLinksTo
	}
	normalized := Edge{From: from, To: to, Predicate: predicate}
	if slices.Contains(g.outgoing[from], normalized) {
		return
	}
	g.nodes[from] = struct{}{}
	g.nodes[to] = struct{}{}
	g.outgoing[from] = append(g.outgoing[from], normalized)
	g.incoming[to] = append(g.incoming[to], normalized)
}

// HasNode reports whether the ID is a node of the graph.
func (g *Graph) HasNode(id string) bool {
	if g == nil {
		return false
	}
	_, exists := g.nodes[id]
	return exists
}

// NodeCount returns the number of nodes.
func (g *Graph) NodeCount() int {
	if g == nil {
		return 0
	}
	return len(g.nodes)
}

// EdgeCount returns the number of stored (deduplicated) edges.
func (g *Graph) EdgeCount() int {
	if g == nil {
		return 0
	}
	total := 0
	for _, edges := range g.outgoing {
		total += len(edges)
	}
	return total
}

// Nodes returns all node IDs sorted ascending.
func (g *Graph) Nodes() []string {
	if g == nil {
		return []string{}
	}
	result := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

// Outgoing returns a copy of the edges leaving the node, sorted by
// (To, Predicate).
func (g *Graph) Outgoing(id string) []Edge {
	if g == nil {
		return []Edge{}
	}
	return append([]Edge{}, g.outgoing[id]...)
}

// Incoming returns a copy of the edges entering the node, sorted by
// (From, To, Predicate).
func (g *Graph) Incoming(id string) []Edge {
	if g == nil {
		return []Edge{}
	}
	return append([]Edge{}, g.incoming[id]...)
}

// Neighbors returns the deduplicated edges touching the node in the given
// direction, sorted by (From, To, Predicate). An empty direction means
// DirectionOutgoing.
func (g *Graph) Neighbors(id string, direction Direction) []Edge {
	if g == nil {
		return []Edge{}
	}
	switch direction {
	case DirectionIncoming:
		return g.Incoming(id)
	case DirectionBoth:
		merged := append([]Edge{}, g.outgoing[id]...)
		for _, edge := range g.incoming[id] {
			if edge.From == edge.To {
				continue // self-loop already present from outgoing
			}
			merged = append(merged, edge)
		}
		sortEdges(merged)
		return merged
	default:
		return g.Outgoing(id)
	}
}

func sortEdges(edges []Edge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Predicate < edges[j].Predicate
	})
}
