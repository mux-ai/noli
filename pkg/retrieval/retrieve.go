package retrieval

import (
	"errors"
	"strings"

	"noli/pkg/graph"
	"noli/pkg/search"
)

// ErrContextLimitTooSmall reports that not even the first source header fits
// into MaxCharacters. The CLI maps it to CONTEXT_LIMIT_TOO_SMALL.
var ErrContextLimitTooSmall = errors.New("maximum characters cannot fit a single source header")

// Source is one document included in the assembled context.
type Source struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	// Seed marks search seeds; graph-only documents have Seed false.
	Seed bool `json:"seed"`
	// Score is the integer search score; zero for graph-only documents.
	Score int `json:"score"`
	// Distance is the number of hops from the originating seed.
	Distance int `json:"distance"`
	// Predecessor is the document whose edge first reached this one.
	Predecessor string `json:"predecessor"`
	// Relationship is the predicate of the originating edge.
	Relationship string `json:"relationship"`
	// Truncated marks a source whose content was cut at the budget.
	Truncated bool `json:"truncated"`
}

// Statistics describes the assembled context.
type Statistics struct {
	SeedCount      int `json:"seed_count"`
	GraphCount     int `json:"graph_count"`
	DocumentCount  int `json:"document_count"`
	CharacterCount int `json:"character_count"`
	MaxCharacters  int `json:"max_characters"`
	// Truncated is true when any source was cut or dropped for budget.
	Truncated bool `json:"truncated"`
}

// Result is a completed retrieval.
type Result struct {
	Query      string     `json:"query"`
	Context    string     `json:"context"`
	Sources    []Source   `json:"sources"`
	Statistics Statistics `json:"statistics"`
}

// Retrieve searches for ranked seeds, expands the graph around them, selects
// documents deterministically, and assembles a bounded context.
//
// Frozen ordering (docs/PROTOCOL.md section 6): seeds by score descending
// then ID; graph-only documents by distance, originating seed rank,
// relationship, then ID; seeds always precede graph-only documents. Every
// record ID must be a node of the graph.
func Retrieve(query string, records []Record, g *graph.Graph, index *search.Index, options Options) (Result, error) {
	options, err := options.withDefaults()
	if err != nil {
		return Result{}, err
	}
	result := Result{
		Query:      query,
		Sources:    []Source{},
		Statistics: Statistics{MaxCharacters: options.MaxCharacters},
	}
	recordsByID := make(map[string]Record, len(records))
	for _, record := range records {
		recordsByID[record.ID] = record
	}

	seedResults := index.Search(query, search.Options{
		Limit:        options.SearchLimit,
		IncludeTypes: options.IncludeTypes,
		ExcludeTypes: options.ExcludeTypes,
	})
	if len(seedResults) == 0 {
		return result, nil
	}
	seedIDs := make([]string, len(seedResults))
	seedScores := make(map[string]int, len(seedResults))
	for i, seed := range seedResults {
		seedIDs[i] = seed.ID
		seedScores[seed.ID] = seed.Score
	}

	visits := g.Expand(seedIDs, graph.ExpandOptions{
		MaxHops:   options.MaxHops,
		Direction: options.Direction,
		// Unlimited here: the document cap applies after type filtering so
		// filtered-out candidates do not waste selection slots.
		MaxDocuments: 0,
	})

	include := typeSet(options.IncludeTypes)
	exclude := typeSet(options.ExcludeTypes)
	selected := make([]selection, 0, options.MaxDocuments)
	for _, visit := range visits {
		if len(selected) >= options.MaxDocuments {
			break
		}
		record, exists := recordsByID[visit.ID]
		if !exists || record.Navigation || record.Log {
			continue
		}
		_, isSeed := seedScores[visit.ID]
		if !isSeed && !typeAllowed(record.Type, include, exclude) {
			continue
		}
		selected = append(selected, selection{
			record: record,
			source: Source{
				ID:           visit.ID,
				Type:         record.Type,
				Title:        record.Title,
				Seed:         isSeed && visit.Distance == 0,
				Score:        seedScores[visit.ID],
				Distance:     visit.Distance,
				Predecessor:  visit.Predecessor,
				Relationship: visit.Relationship,
			},
		})
	}
	if len(selected) == 0 {
		return result, nil
	}
	return assembleContext(query, selected, options.MaxCharacters)
}

type selection struct {
	record Record
	source Source
}

func typeSet(types []string) map[string]struct{} {
	result := make(map[string]struct{}, len(types))
	for _, value := range types {
		if normalized := strings.ToLower(strings.TrimSpace(value)); normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	return result
}

func typeAllowed(recordType string, include, exclude map[string]struct{}) bool {
	normalized := strings.ToLower(strings.TrimSpace(recordType))
	if len(include) > 0 {
		if _, ok := include[normalized]; !ok {
			return false
		}
	}
	_, excluded := exclude[normalized]
	return !excluded
}
