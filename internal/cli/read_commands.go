package cli

import (
	"errors"
	"fmt"
	"sort"

	"noli/pkg/okf"
	"noli/pkg/protocol"
	"noli/pkg/retrieval"
)

// invalidArgument renders a flag or argument problem.
func (a *App) invalidArgument(command string, err error) int {
	var typed *flagError
	if errors.As(err, &typed) {
		return a.failure(command, protocol.CodeInvalidArgument, typed.message, typed.details())
	}
	return a.failure(command, protocol.CodeInvalidArgument, err.Error(), nil)
}

// loadStore validates the root and loads the bundle, rendering any failure.
// The boolean reports success; on failure the exit code is returned.
func (a *App) loadStore(command, root string) (*okf.Store, int, bool) {
	if code, message, details, ok := checkRoot(root); !ok {
		return nil, a.failure(command, code, message, details), false
	}
	store, err := okf.Load(root)
	if err != nil {
		return nil, a.loadFailure(command, err), false
	}
	return store, 0, true
}

// loadFailure maps SDK load errors onto the frozen protocol codes.
func (a *App) loadFailure(command string, err error) int {
	var aggregate *okf.ParseErrors
	if errors.As(err, &aggregate) && len(aggregate.Problems) > 0 {
		first := aggregate.Problems[0]
		code := protocol.CodeParseError
		if first.Code == okf.CodeInvalidFrontmatter {
			code = protocol.CodeInvalidFrontmatter
		}
		message := first.Message
		if first.Document != "" {
			message = first.Document + ": " + message
		}
		if extra := len(aggregate.Problems) - 1; extra > 0 {
			message = fmt.Sprintf("%s (and %d more problems)", message, extra)
		}
		details := map[string]string{}
		if first.Document != "" {
			details["document"] = first.Document
		}
		return a.failure(command, code, message, details)
	}
	return a.failure(command, protocol.CodeParseError, err.Error(), nil)
}

func (a *App) runStatus(args []string) int {
	fs := newFlagSet("status")
	root := fs.String("root", "", "knowledge root directory")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("status", err)
	}
	store, exit, ok := a.loadStore("status", *root)
	if !ok {
		return exit
	}
	counts := make(map[string]int)
	links := 0
	for _, id := range store.IDs() {
		document, _ := store.Get(id)
		links += len(document.Links)
		if document.IsIndex || document.IsLog {
			continue
		}
		counts[document.Metadata.Type]++
	}
	types := make([]protocol.TypeCount, 0, len(counts))
	for name, count := range counts {
		types = append(types, protocol.TypeCount{Type: name, Count: count})
	}
	sort.Slice(types, func(i, j int) bool { return types[i].Type < types[j].Type })
	return a.success("status", protocol.StatusData{
		Root:          *root,
		BundleID:      store.BundleID(),
		DocumentCount: store.DocumentCount(),
		LinkCount:     links,
		Types:         types,
	})
}

func (a *App) runList(args []string) int {
	fs := newFlagSet("list")
	root := fs.String("root", "", "knowledge root directory")
	typesFlag := fs.String("types", "", "comma-separated type filter")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("list", err)
	}
	store, exit, ok := a.loadStore("list", *root)
	if !ok {
		return exit
	}
	documents := store.List(okf.ListOptions{Types: splitTypes(*typesFlag)})
	summaries := make([]protocol.DocumentSummary, 0, len(documents))
	for _, document := range documents {
		summaries = append(summaries, protocol.DocumentSummary{
			ID:          document.ID,
			Type:        document.Metadata.Type,
			Title:       document.Metadata.Title,
			Description: document.Metadata.Description,
			Tags:        nonNilStrings(document.Metadata.Tags),
		})
	}
	return a.success("list", protocol.ListData{Count: len(summaries), Documents: summaries})
}

func (a *App) runSearch(args []string) int {
	fs := newFlagSet("search")
	root := fs.String("root", "", "knowledge root directory")
	query := fs.String("query", "", "search query")
	limit := fs.Int("limit", 0, "maximum results (default 10)")
	typesFlag := fs.String("types", "", "comma-separated type filter")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("search", err)
	}
	if err := requireValue("--query", *query); err != nil {
		return a.invalidArgument("search", err)
	}
	if err := requireNonNegative("--limit", *limit); err != nil {
		return a.invalidArgument("search", err)
	}
	store, exit, ok := a.loadStore("search", *root)
	if !ok {
		return exit
	}
	results := store.Search(*query, okf.SearchOptions{
		Limit:        *limit,
		IncludeTypes: splitTypes(*typesFlag),
	})
	items := make([]protocol.SearchResultItem, 0, len(results))
	for _, result := range results {
		items = append(items, protocol.SearchResultItem{
			ID:          result.ID,
			Type:        result.Type,
			Title:       result.Title,
			Description: result.Description,
			Score:       result.Score,
		})
	}
	return a.success("search", protocol.SearchData{Query: *query, Count: len(items), Results: items})
}

func (a *App) runRetrieve(args []string) int {
	fs := newFlagSet("retrieve")
	root := fs.String("root", "", "knowledge root directory")
	query := fs.String("query", "", "retrieval question")
	typesFlag := fs.String("types", "", "comma-separated type filter")
	searchLimit := fs.Int("search-limit", 0, "maximum seeds (default 5)")
	maxHops := fs.Int("max-hops", 0, "maximum graph hops (default 1)")
	maxDocuments := fs.Int("max-documents", 0, "maximum documents (default 10)")
	maxCharacters := fs.Int("max-characters", 0, "maximum context characters (default 12000)")
	direction := fs.String("direction", "both", "graph direction (outgoing, incoming, both)")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("retrieve", err)
	}
	if err := requireValue("--query", *query); err != nil {
		return a.invalidArgument("retrieve", err)
	}
	for _, bound := range []struct {
		name  string
		value int
	}{
		{"--search-limit", *searchLimit},
		{"--max-hops", *maxHops},
		{"--max-documents", *maxDocuments},
		{"--max-characters", *maxCharacters},
	} {
		if err := requireNonNegative(bound.name, bound.value); err != nil {
			return a.invalidArgument("retrieve", err)
		}
	}
	parsedDirection, err := parseDirection(*direction)
	if err != nil {
		return a.invalidArgument("retrieve", err)
	}
	store, exit, ok := a.loadStore("retrieve", *root)
	if !ok {
		return exit
	}
	result, err := store.Retrieve(*query, okf.RetrieveOptions{
		SearchLimit:   *searchLimit,
		MaxHops:       *maxHops,
		MaxDocuments:  *maxDocuments,
		MaxCharacters: *maxCharacters,
		Direction:     parsedDirection,
		IncludeTypes:  splitTypes(*typesFlag),
	})
	if err != nil {
		if errors.Is(err, retrieval.ErrContextLimitTooSmall) {
			return a.failure("retrieve", protocol.CodeContextLimitTooSmall, err.Error(),
				map[string]string{"max_characters": fmt.Sprint(*maxCharacters)})
		}
		return a.failure("retrieve", protocol.CodeInternalError, err.Error(), nil)
	}
	sources := make([]protocol.RetrieveSource, 0, len(result.Sources))
	for _, source := range result.Sources {
		sources = append(sources, protocol.RetrieveSource{
			ID:           source.ID,
			Type:         source.Type,
			Title:        source.Title,
			Seed:         source.Seed,
			Score:        source.Score,
			Distance:     source.Distance,
			Predecessor:  source.Predecessor,
			Relationship: source.Relationship,
			Truncated:    source.Truncated,
		})
	}
	return a.success("retrieve", protocol.RetrieveData{
		Query:   result.Query,
		Context: result.Context,
		Sources: sources,
		Statistics: protocol.RetrieveStatistics{
			SeedCount:      result.Statistics.SeedCount,
			GraphCount:     result.Statistics.GraphCount,
			DocumentCount:  result.Statistics.DocumentCount,
			CharacterCount: result.Statistics.CharacterCount,
			MaxCharacters:  result.Statistics.MaxCharacters,
			Truncated:      result.Statistics.Truncated,
		},
	})
}

func (a *App) runGet(args []string) int {
	fs := newFlagSet("get")
	root := fs.String("root", "", "knowledge root directory")
	id := fs.String("id", "", "document ID")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("get", err)
	}
	if err := requireValue("--id", *id); err != nil {
		return a.invalidArgument("get", err)
	}
	store, exit, ok := a.loadStore("get", *root)
	if !ok {
		return exit
	}
	document, found := store.Get(*id)
	if !found {
		return a.failure("get", protocol.CodeDocumentNotFound,
			fmt.Sprintf("document %q was not found", *id), map[string]string{"id": *id})
	}
	links := make([]protocol.DocumentLink, 0, len(document.Links))
	for _, link := range document.Links {
		links = append(links, protocol.DocumentLink{Target: link.Target, Predicate: link.Predicate})
	}
	metadata := document.Metadata.Extra
	if metadata == nil {
		metadata = map[string]any{}
	}
	return a.success("get", protocol.GetData{Document: protocol.DocumentDetail{
		ID:          document.ID,
		Type:        document.Metadata.Type,
		Title:       document.Metadata.Title,
		Description: document.Metadata.Description,
		Tags:        nonNilStrings(document.Metadata.Tags),
		Metadata:    metadata,
		Links:       links,
		Body:        document.Body,
	}})
}

func (a *App) runGraph(args []string) int {
	fs := newFlagSet("graph")
	root := fs.String("root", "", "knowledge root directory")
	id := fs.String("id", "", "document ID")
	direction := fs.String("direction", "both", "graph direction (outgoing, incoming, both)")
	maxHops := fs.Int("max-hops", 1, "maximum hops (default 1)")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("graph", err)
	}
	if err := requireValue("--id", *id); err != nil {
		return a.invalidArgument("graph", err)
	}
	if err := requireNonNegative("--max-hops", *maxHops); err != nil {
		return a.invalidArgument("graph", err)
	}
	parsedDirection, err := parseDirection(*direction)
	if err != nil {
		return a.invalidArgument("graph", err)
	}
	store, exit, ok := a.loadStore("graph", *root)
	if !ok {
		return exit
	}
	view, err := store.GraphView(*id, okf.GraphOptions{Direction: parsedDirection, MaxHops: *maxHops})
	if err != nil {
		return a.failure("graph", protocol.CodeDocumentNotFound,
			fmt.Sprintf("document %q was not found", *id), map[string]string{"id": *id})
	}
	nodes := make([]protocol.GraphNodeData, 0, len(view.Nodes))
	for _, node := range view.Nodes {
		nodes = append(nodes, protocol.GraphNodeData{
			ID:           node.ID,
			Distance:     node.Distance,
			Predecessor:  node.Predecessor,
			Relationship: node.Relationship,
		})
	}
	edges := make([]protocol.GraphEdgeData, 0, len(view.Edges))
	for _, edge := range view.Edges {
		edges = append(edges, protocol.GraphEdgeData{From: edge.From, To: edge.To, Predicate: edge.Predicate})
	}
	return a.success("graph", protocol.GraphData{
		ID:        view.ID,
		Direction: view.Direction,
		MaxHops:   view.MaxHops,
		Nodes:     nodes,
		Edges:     edges,
	})
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
