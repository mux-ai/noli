// Package search provides deterministic keyword search over
// package-independent document records. It never imports OKF document types;
// callers adapt their models into Record values.
package search

import (
	"sort"
	"strings"
)

// Record is a searchable document record, independent of any document
// package.
type Record struct {
	ID          string
	Type        string
	Title       string
	Description string
	Tags        []string
	Metadata    map[string]any
	Body        string
	// Navigation marks index/navigation documents, excluded by default.
	Navigation bool
	// Log marks log documents, excluded by default.
	Log bool
}

// Options bounds and filters a search.
type Options struct {
	// Limit caps the number of results. Zero means no cap; negative returns
	// no results.
	Limit int
	// IncludeTypes, when non-empty, restricts results to these types
	// (case-insensitive, trimmed).
	IncludeTypes []string
	// ExcludeTypes drops records of these types (case-insensitive, trimmed).
	ExcludeTypes []string
	// IncludeNavigation includes navigation/index documents.
	IncludeNavigation bool
	// IncludeLog includes log documents.
	IncludeLog bool
}

// Result is one scored search hit. Scores are integers per the frozen
// protocol contract.
type Result struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Score       int    `json:"score"`
}

// Field weights per unique matched query token, and the per-field exact
// normalized phrase bonus. Frozen in docs/PROTOCOL.md section 5.
const (
	weightTitle       = 5
	weightDescription = 3
	weightTags        = 2
	weightMetadata    = 2
	weightBody        = 1
	phraseBonus       = 1
)

// Index is an immutable keyword index over records.
type Index struct {
	records []Record
}

// NewIndex copies the records and orders them by ID for deterministic
// scanning.
func NewIndex(records []Record) *Index {
	copied := append([]Record(nil), records...)
	sort.Slice(copied, func(i, j int) bool { return copied[i].ID < copied[j].ID })
	return &Index{records: copied}
}

// Search scores every eligible record against the query. Results are sorted
// by score descending, then ID ascending. Records with zero score are
// omitted. The returned slice is never nil.
func (index *Index) Search(query string, options Options) []Result {
	results := make([]Result, 0)
	if index == nil || options.Limit < 0 {
		return results
	}
	queryTokens := uniqueTokens(tokenize(query))
	if len(queryTokens) == 0 {
		return results
	}
	phrase := normalizePhrase(query)
	include := typeSet(options.IncludeTypes)
	exclude := typeSet(options.ExcludeTypes)
	for _, record := range index.records {
		if (record.Navigation && !options.IncludeNavigation) || (record.Log && !options.IncludeLog) {
			continue
		}
		if !typeAllowed(record.Type, include, exclude) {
			continue
		}
		score := scoreRecord(record, queryTokens, phrase)
		if score > 0 {
			results = append(results, Result{
				ID:          record.ID,
				Type:        record.Type,
				Title:       record.Title,
				Description: record.Description,
				Score:       score,
			})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ID < results[j].ID
	})
	if options.Limit > 0 && len(results) > options.Limit {
		results = results[:options.Limit]
	}
	return results
}

type weightedField struct {
	text   string
	weight int
}

func scoreRecord(record Record, queryTokens []string, phrase string) int {
	fields := []weightedField{
		{text: record.Title, weight: weightTitle},
		{text: record.Description, weight: weightDescription},
		{text: strings.Join(record.Tags, " "), weight: weightTags},
		{text: metadataText(record.Metadata), weight: weightMetadata},
		{text: record.Body, weight: weightBody},
	}
	score := 0
	for _, field := range fields {
		fieldTokens := tokenSet(tokenize(field.text))
		for _, token := range queryTokens {
			if _, exists := fieldTokens[token]; exists {
				score += field.weight
			}
		}
		if phrase != "" && strings.Contains(normalizePhrase(field.text), phrase) {
			score += phraseBonus
		}
	}
	return score
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
