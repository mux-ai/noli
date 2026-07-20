package okf

import (
	"sort"
	"strings"

	"noli/pkg/graph"
	"noli/pkg/search"
)

// Store is an immutable, loaded knowledge bundle. All accessors return
// copies; mutating a returned document never affects the Store.
type Store struct {
	root      string
	documents map[string]*Document
	order     []string
	graph     *graph.Graph
	index     *search.Index
	bundleID  string
}

// ListOptions filters List results.
type ListOptions struct {
	// Types, when non-empty, restricts results to these types
	// (case-insensitive, trimmed).
	Types []string
	// IncludeNavigation includes index documents.
	IncludeNavigation bool
	// IncludeLog includes log documents.
	IncludeLog bool
}

// Load parses the knowledge bundle below root with default bounds and builds
// an immutable Store. Any parse failure fails the whole load; use Validate to
// enumerate all problems.
func Load(root string) (*Store, error) {
	return LoadWithOptions(root, ParseOptions{})
}

// LoadWithOptions is Load with explicit parser bounds and exclusions.
func LoadWithOptions(root string, options ParseOptions) (*Store, error) {
	bundle, err := ParseBundle(root, options)
	if err != nil {
		return nil, err
	}
	documents := make(map[string]*Document, len(bundle.Documents))
	order := append([]string(nil), bundle.Order...)
	sort.Strings(order)

	nodes := make([]string, 0, len(order))
	var edges []graph.Edge
	records := make([]search.Record, 0, len(order))
	for _, id := range order {
		document := bundle.Documents[id]
		stored := document.Clone()
		documents[id] = &stored
		nodes = append(nodes, id)
	}
	for _, id := range order {
		document := documents[id]
		for _, link := range document.Links {
			if _, exists := documents[link.Target]; !exists {
				continue // broken links are a validation concern
			}
			edges = append(edges, graph.Edge{From: id, To: link.Target, Predicate: link.Predicate})
		}
		records = append(records, search.Record{
			ID:          document.ID,
			Type:        document.Metadata.Type,
			Title:       document.Metadata.Title,
			Description: document.Metadata.Description,
			Tags:        document.Metadata.Tags,
			Metadata:    document.Metadata.Extra,
			Body:        document.Body,
			Navigation:  document.IsIndex,
			Log:         document.IsLog,
		})
	}
	return &Store{
		root:      bundle.Root,
		documents: documents,
		order:     order,
		graph:     graph.New(nodes, edges),
		index:     search.NewIndex(records),
		bundleID:  bundle.BundleID,
	}, nil
}

// Root returns the absolute, symlink-resolved knowledge root.
func (s *Store) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

// BundleID returns "sha256:<hex>" over sorted document IDs and normalized
// document bytes. It versions the loaded bundle.
func (s *Store) BundleID() string {
	if s == nil {
		return ""
	}
	return s.bundleID
}

// DocumentCount returns the number of loaded documents.
func (s *Store) DocumentCount() int {
	if s == nil {
		return 0
	}
	return len(s.order)
}

// IDs returns all document IDs sorted ascending.
func (s *Store) IDs() []string {
	if s == nil {
		return []string{}
	}
	return append([]string{}, s.order...)
}

// Get returns a copy of the document with the given ID.
func (s *Store) Get(id string) (*Document, bool) {
	if s == nil {
		return nil, false
	}
	document, ok := s.documents[id]
	if !ok {
		return nil, false
	}
	clone := document.Clone()
	return &clone, true
}

// List returns copies of the documents in ID order, filtered by options.
// Navigation and log documents are excluded unless explicitly included.
func (s *Store) List(options ListOptions) []*Document {
	result := make([]*Document, 0)
	if s == nil {
		return result
	}
	types := make(map[string]struct{}, len(options.Types))
	for _, value := range options.Types {
		if normalized := strings.ToLower(strings.TrimSpace(value)); normalized != "" {
			types[normalized] = struct{}{}
		}
	}
	for _, id := range s.order {
		document := s.documents[id]
		if (document.IsIndex && !options.IncludeNavigation) || (document.IsLog && !options.IncludeLog) {
			continue
		}
		if len(types) > 0 {
			if _, ok := types[strings.ToLower(strings.TrimSpace(document.Metadata.Type))]; !ok {
				continue
			}
		}
		clone := document.Clone()
		result = append(result, &clone)
	}
	return result
}
