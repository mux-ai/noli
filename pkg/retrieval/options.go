// Package retrieval combines keyword search seeds with bounded graph
// expansion and assembles a deterministic, budgeted agent context. It works
// with package-independent records plus pkg/graph and pkg/search; it never
// imports the OKF document package.
package retrieval

import (
	"errors"

	"noli/pkg/graph"
)

// Frozen defaults (docs/PROTOCOL.md section 4), applied when the
// corresponding option is zero.
const (
	DefaultSearchLimit   = 5
	DefaultMaxHops       = 1
	DefaultMaxDocuments  = 10
	DefaultMaxCharacters = 12000
)

// Record is a package-independent retrieval document record. Content is the
// text placed into the assembled context (for OKF documents, the Markdown
// body).
type Record struct {
	ID          string
	Type        string
	Title       string
	Description string
	Tags        []string
	Metadata    map[string]any
	Content     string
	// Navigation and Log documents are never selected.
	Navigation bool
	Log        bool
}

// Options bounds a retrieval. Zero values select the frozen defaults;
// negative values are invalid. MaxCharacters counts runes.
type Options struct {
	SearchLimit   int
	MaxHops       int
	MaxDocuments  int
	MaxCharacters int
	// Direction selects graph expansion edges. Empty means both.
	Direction graph.Direction
	// IncludeTypes/ExcludeTypes apply to search seeds and to expanded
	// candidates (case-insensitive, trimmed).
	IncludeTypes []string
	ExcludeTypes []string
}

// Normalized returns the options with frozen defaults applied, or an error
// for negative bounds or an unknown direction.
func (o Options) Normalized() (Options, error) {
	return o.withDefaults()
}

func (o Options) withDefaults() (Options, error) {
	if o.SearchLimit < 0 || o.MaxHops < 0 || o.MaxDocuments < 0 || o.MaxCharacters < 0 {
		return o, errors.New("retrieval bounds must not be negative")
	}
	if o.SearchLimit == 0 {
		o.SearchLimit = DefaultSearchLimit
	}
	if o.MaxHops == 0 {
		o.MaxHops = DefaultMaxHops
	}
	if o.MaxDocuments == 0 {
		o.MaxDocuments = DefaultMaxDocuments
	}
	if o.MaxCharacters == 0 {
		o.MaxCharacters = DefaultMaxCharacters
	}
	switch o.Direction {
	case "":
		o.Direction = graph.DirectionBoth
	case graph.DirectionOutgoing, graph.DirectionIncoming, graph.DirectionBoth:
	default:
		return o, errors.New("direction must be outgoing, incoming, or both")
	}
	return o, nil
}
