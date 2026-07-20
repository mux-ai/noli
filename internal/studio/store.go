package studio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"noli/internal/canonicalize"
	"noli/internal/llm"
	"noli/internal/okf"
	"noli/internal/project"
	"noli/pkg/graph"
	"noli/pkg/retrieval"
	"noli/pkg/search"
)

const KnowledgeAssistantSystemPrompt = `You are a local knowledge assistant.

Use only the supplied knowledge context.

Do not invent facts.

Cite important statements using [source: document-id].

If the supplied knowledge is insufficient, say so clearly.

Clearly distinguish source-supported facts from uncertainty.`

// LoadedStore combines the parsed Studio bundle with the public toolkit
// engine (pkg/graph, pkg/search, pkg/retrieval).
type LoadedStore struct {
	Bundle  *okf.ParsedBundle
	Graph   *graph.Graph
	Index   *search.Index
	records []retrieval.Record
}

// Search runs the public keyword engine over the loaded documents.
func (s LoadedStore) Search(query string, limit int) []search.Result {
	return s.Index.Search(query, search.Options{Limit: limit})
}

// LoadStore parses Markdown and, when reviewed canonical staging data is
// available, restores typed relationship edges in addition to Markdown
// links. Typed edge confidence and evidence are staging-review concerns and
// are not carried into the runtime graph.
func LoadStore(workspace project.Workspace, profile *project.ProjectProfile) (LoadedStore, error) {
	bundle, err := okf.ParseBundle(workspace.KnowledgeDir)
	if err != nil {
		return LoadedStore{}, fmt.Errorf("load knowledge store for %s: %w", workspace.Config.Name, err)
	}
	nodes := append([]string(nil), bundle.Order...)
	known := make(map[string]struct{}, len(bundle.Order))
	for _, id := range bundle.Order {
		known[id] = struct{}{}
	}
	var edges []graph.Edge
	for _, id := range bundle.Order {
		for _, target := range bundle.Documents[id].Links {
			if _, exists := known[target]; exists {
				edges = append(edges, graph.Edge{From: id, To: target, Predicate: graph.PredicateLinksTo})
			}
		}
	}
	concepts, conceptErr := LoadCanonicalConcepts(workspace.CanonicalConceptsPath)
	if conceptErr == nil && profile != nil {
		resolved, _ := canonicalize.ResolveRelationships(*profile, concepts)
		for _, relation := range resolved {
			edges = append(edges, graph.Edge{From: relation.From, To: relation.To, Predicate: relation.Predicate})
		}
	} else if conceptErr != nil && !errors.Is(conceptErr, os.ErrNotExist) {
		return LoadedStore{}, fmt.Errorf("load knowledge store for %s: %w", workspace.Config.Name, conceptErr)
	}

	records := make([]retrieval.Record, 0, len(bundle.Order))
	searchRecords := make([]search.Record, 0, len(bundle.Order))
	for _, id := range bundle.Order {
		document := bundle.Documents[id]
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
		searchRecords = append(searchRecords, search.Record{
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
	return LoadedStore{
		Bundle:  bundle,
		Graph:   graph.New(nodes, edges),
		Index:   search.NewIndex(searchRecords),
		records: records,
	}, nil
}

type AnswerResult struct {
	Answer      string
	SelectedIDs []string
	Search      []search.Result
}

func Ask(ctx context.Context, client llm.Client, workspace project.Workspace, profile project.ProjectProfile, question string, contextLimit int) (AnswerResult, error) {
	if client == nil {
		return AnswerResult{}, fmt.Errorf("answer question: LLM client is required")
	}
	question = strings.TrimSpace(question)
	if question == "" {
		return AnswerResult{}, fmt.Errorf("answer question: question must not be empty")
	}
	if contextLimit <= 0 {
		return AnswerResult{}, fmt.Errorf("answer question: context limit must be positive")
	}
	loaded, err := LoadStore(workspace, &profile)
	if err != nil {
		return AnswerResult{}, err
	}
	results := loaded.Search(question, 8)
	if len(results) == 0 {
		return AnswerResult{Answer: "The supplied knowledge is insufficient to answer that question."}, nil
	}
	retrieved, err := retrieval.Retrieve(question, loaded.records, loaded.Graph, loaded.Index, retrieval.Options{
		SearchLimit:   8,
		MaxHops:       1,
		MaxDocuments:  12,
		MaxCharacters: contextLimit,
		Direction:     graph.DirectionBoth,
	})
	if err != nil {
		return AnswerResult{}, fmt.Errorf("build answer context: %w", err)
	}
	if strings.TrimSpace(retrieved.Context) == "" {
		return AnswerResult{Answer: "The supplied knowledge is insufficient to answer that question.", Search: results}, nil
	}
	systemPrompt := KnowledgeAssistantSystemPrompt
	if safety := joinedSafetyInstructions(profile); safety != "" {
		systemPrompt += "\n\nPROJECT-SPECIFIC SAFETY INSTRUCTIONS:\n" + safety
	}
	answer, err := client.Chat(ctx, systemPrompt, question, retrieved.Context)
	if err != nil {
		return AnswerResult{}, fmt.Errorf("answer question with Ollama: %w", err)
	}
	selected := make([]string, 0, len(retrieved.Sources))
	for _, source := range retrieved.Sources {
		selected = append(selected, source.ID)
	}
	return AnswerResult{Answer: answer, SelectedIDs: selected, Search: results}, nil
}
