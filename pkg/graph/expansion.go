package graph

import (
	"sort"
	"strings"
)

// ExpandOptions bounds a breadth-first expansion.
type ExpandOptions struct {
	// MaxHops is the maximum distance from a seed. Zero or negative means
	// seeds only.
	MaxHops int
	// MaxDocuments caps the total number of visits, seeds included. Zero or
	// negative means no cap.
	MaxDocuments int
	// Direction selects which edges are followed. Empty means outgoing.
	Direction Direction
	// Predicates optionally restricts traversal to edges whose predicate
	// matches one of these values (case-insensitive, trimmed). Empty means
	// all predicates.
	Predicates []string
}

// Visit is one document reached by Expand.
type Visit struct {
	// ID is the document ID.
	ID string `json:"id"`
	// Distance is the number of hops from the originating seed; zero for
	// seeds.
	Distance int `json:"distance"`
	// Predecessor is the ID of the node whose edge first reached this node;
	// empty for seeds.
	Predecessor string `json:"predecessor"`
	// Relationship is the predicate of the originating edge; empty for seeds.
	Relationship string `json:"relationship"`
	// SeedRank is the zero-based rank of the seed whose expansion first
	// reached this node. Seeds carry their own rank.
	SeedRank int `json:"seed_rank"`
}

// Expand performs a deterministic breadth-first expansion from the seeds.
//
// Caller seed order is preserved exactly: seeds are deduplicated keeping the
// first occurrence and are never re-sorted. Blank seeds and seeds absent from
// the graph are skipped. Seeds always precede graph-only visits. Graph-only
// visits are ordered by distance ascending, then originating seed rank, then
// relationship, then ID. Every node is visited at most once.
func (g *Graph) Expand(seeds []string, options ExpandOptions) []Visit {
	visits := make([]Visit, 0, len(seeds))
	if g == nil {
		return visits
	}
	direction := options.Direction
	if direction == "" {
		direction = DirectionOutgoing
	}
	limit := options.MaxDocuments
	if limit <= 0 {
		limit = len(g.nodes)
	}
	seen := make(map[string]struct{}, len(seeds))
	for _, seed := range seeds {
		id := strings.TrimSpace(seed)
		if id == "" || !g.HasNode(id) {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		if len(visits) >= limit {
			break
		}
		seen[id] = struct{}{}
		visits = append(visits, Visit{ID: id, SeedRank: len(visits)})
	}
	allowed := make(map[string]struct{}, len(options.Predicates))
	for _, predicate := range options.Predicates {
		if normalized := strings.ToLower(strings.TrimSpace(predicate)); normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}
	frontier := append([]Visit{}, visits...)
	for depth := 0; depth < options.MaxHops && len(visits) < limit && len(frontier) > 0; depth++ {
		candidates := make([]Visit, 0)
		for _, visit := range frontier {
			for _, edge := range g.traversal(visit.ID, direction, allowed) {
				other := edge.To
				if other == visit.ID {
					other = edge.From
				}
				if _, exists := seen[other]; exists {
					continue
				}
				candidates = append(candidates, Visit{
					ID:           other,
					Distance:     depth + 1,
					Predecessor:  visit.ID,
					Relationship: edge.Predicate,
					SeedRank:     visit.SeedRank,
				})
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].SeedRank != candidates[j].SeedRank {
				return candidates[i].SeedRank < candidates[j].SeedRank
			}
			if candidates[i].Relationship != candidates[j].Relationship {
				return candidates[i].Relationship < candidates[j].Relationship
			}
			return candidates[i].ID < candidates[j].ID
		})
		frontier = frontier[:0]
		for _, candidate := range candidates {
			if _, exists := seen[candidate.ID]; exists {
				continue
			}
			if len(visits) >= limit {
				break
			}
			seen[candidate.ID] = struct{}{}
			visits = append(visits, candidate)
			frontier = append(frontier, candidate)
		}
	}
	return visits
}

// traversal returns the edges leaving id in the requested direction, filtered
// by the allowed predicate set (empty set means all). For DirectionIncoming
// and DirectionBoth the returned edges may point at id; callers derive the
// neighbor from whichever endpoint differs from id.
func (g *Graph) traversal(id string, direction Direction, allowed map[string]struct{}) []Edge {
	match := func(edge Edge) bool {
		if len(allowed) == 0 {
			return true
		}
		_, exists := allowed[strings.ToLower(edge.Predicate)]
		return exists
	}
	result := make([]Edge, 0)
	if direction == DirectionOutgoing || direction == DirectionBoth {
		for _, edge := range g.outgoing[id] {
			if match(edge) {
				result = append(result, edge)
			}
		}
	}
	if direction == DirectionIncoming || direction == DirectionBoth {
		for _, edge := range g.incoming[id] {
			if edge.From == edge.To {
				continue // self-loop, already handled or irrelevant
			}
			if match(edge) {
				result = append(result, edge)
			}
		}
	}
	return result
}
