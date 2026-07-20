package okf

import (
	"fmt"
	"sort"

	"noli/pkg/graph"
	"noli/pkg/retrieval"
	"noli/pkg/search"
)

// SearchResult is one scored hit with an integer score.
type SearchResult = search.Result

// RetrieveResult is a completed retrieval with context, sources, and
// statistics.
type RetrieveResult = retrieval.Result

// SearchOptions bounds and filters Store.Search.
type SearchOptions struct {
	// Limit caps results; zero means the frozen default of 10.
	Limit             int
	IncludeTypes      []string
	ExcludeTypes      []string
	IncludeNavigation bool
	IncludeLog        bool
}

// DefaultSearchLimit is the frozen default for Store.Search
// (docs/PROTOCOL.md section 4).
const DefaultSearchLimit = 10

// RetrieveOptions bounds Store.Retrieve. Zero values select the frozen
// defaults (search limit 5, hops 1, documents 10, characters 12000,
// direction both).
type RetrieveOptions struct {
	SearchLimit   int
	MaxHops       int
	MaxDocuments  int
	MaxCharacters int
	Direction     graph.Direction
	IncludeTypes  []string
	ExcludeTypes  []string
}

// GraphOptions bounds Store.GraphView. Zero values select direction both and
// one hop.
type GraphOptions struct {
	Direction graph.Direction
	MaxHops   int
}

// GraphNode is one document reached by GraphView.
type GraphNode struct {
	ID           string `json:"id"`
	Distance     int    `json:"distance"`
	Predecessor  string `json:"predecessor"`
	Relationship string `json:"relationship"`
}

// GraphResult is the bounded neighborhood of one document.
type GraphResult struct {
	ID        string       `json:"id"`
	Direction string       `json:"direction"`
	MaxHops   int          `json:"max_hops"`
	Nodes     []GraphNode  `json:"nodes"`
	Edges     []graph.Edge `json:"edges"`
}

// Search runs deterministic keyword search over the loaded documents.
func (s *Store) Search(query string, options SearchOptions) []SearchResult {
	if s == nil {
		return []SearchResult{}
	}
	limit := options.Limit
	if limit == 0 {
		limit = DefaultSearchLimit
	}
	results := s.index.Search(query, search.Options{
		Limit:             limit,
		IncludeTypes:      options.IncludeTypes,
		ExcludeTypes:      options.ExcludeTypes,
		IncludeNavigation: options.IncludeNavigation,
		IncludeLog:        options.IncludeLog,
	})
	if results == nil {
		return []SearchResult{}
	}
	return results
}

// Retrieve combines search seeds with graph expansion into a bounded,
// source-traceable context.
func (s *Store) Retrieve(query string, options RetrieveOptions) (RetrieveResult, error) {
	if s == nil {
		return RetrieveResult{}, fmt.Errorf("retrieve: store is nil")
	}
	records := make([]retrieval.Record, 0, len(s.order))
	for _, id := range s.order {
		document := s.documents[id]
		records = append(records, retrieval.Record{
			ID:          document.ID,
			Type:        document.Metadata.Type,
			Title:       document.Metadata.Title,
			Description: document.Metadata.Description,
			Tags:        document.Metadata.Tags,
			Metadata:    document.Metadata.Extra,
			Content:     document.Body,
			Navigation:  document.IsIndex,
			Log:         document.IsLog,
		})
	}
	return retrieval.Retrieve(query, records, s.graph, s.index, retrieval.Options{
		SearchLimit:   options.SearchLimit,
		MaxHops:       options.MaxHops,
		MaxDocuments:  options.MaxDocuments,
		MaxCharacters: options.MaxCharacters,
		Direction:     options.Direction,
		IncludeTypes:  options.IncludeTypes,
		ExcludeTypes:  options.ExcludeTypes,
	})
}

// GraphView returns the bounded neighborhood of one document together with
// the edges between the returned nodes.
func (s *Store) GraphView(id string, options GraphOptions) (GraphResult, error) {
	if s == nil {
		return GraphResult{}, fmt.Errorf("graph view: store is nil")
	}
	if _, exists := s.documents[id]; !exists {
		return GraphResult{}, fmt.Errorf("document %q was not found", id)
	}
	direction := options.Direction
	switch direction {
	case "":
		direction = graph.DirectionBoth
	case graph.DirectionOutgoing, graph.DirectionIncoming, graph.DirectionBoth:
	default:
		return GraphResult{}, fmt.Errorf("direction must be outgoing, incoming, or both")
	}
	maxHops := options.MaxHops
	if maxHops == 0 {
		maxHops = 1
	}
	if maxHops < 0 {
		return GraphResult{}, fmt.Errorf("maximum hops must not be negative")
	}
	visits := s.graph.Expand([]string{id}, graph.ExpandOptions{
		MaxHops:   maxHops,
		Direction: direction,
	})
	nodes := make([]GraphNode, 0, len(visits))
	visited := make(map[string]struct{}, len(visits))
	for _, visit := range visits {
		visited[visit.ID] = struct{}{}
		nodes = append(nodes, GraphNode{
			ID:           visit.ID,
			Distance:     visit.Distance,
			Predecessor:  visit.Predecessor,
			Relationship: visit.Relationship,
		})
	}
	edges := make([]graph.Edge, 0)
	for _, visit := range visits {
		for _, edge := range s.graph.Outgoing(visit.ID) {
			if _, ok := visited[edge.To]; ok {
				edges = append(edges, edge)
			}
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Predicate < edges[j].Predicate
	})
	return GraphResult{
		ID:        id,
		Direction: string(direction),
		MaxHops:   maxHops,
		Nodes:     nodes,
		Edges:     edges,
	}, nil
}
