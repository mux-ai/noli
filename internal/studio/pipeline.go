package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"noli/internal/canonicalize"
	"noli/internal/extract"
	"noli/internal/llm"
	"noli/internal/normalize"
	"noli/internal/okf"
	"noli/internal/project"
	"noli/internal/source"
)

type Engine struct {
	Client              llm.Client
	SourceLoader        source.Loader
	MaximumChunk        int
	RequestTimeout      time.Duration
	Now                 func() time.Time
	StrictRelationships bool
}

type ExtractionResult struct {
	Documents []source.SourceDocument
	Drafts    []extract.ConceptDraft
	Warnings  []source.Warning
}

type GenerationResult struct {
	Documents           []source.SourceDocument
	Drafts              []extract.ConceptDraft
	Concepts            []canonicalize.CanonicalConcept
	Edges               []canonicalize.Edge
	UnresolvedRelations []canonicalize.UnresolvedRelation
	SourceWarnings      []source.Warning
	ValidationProblems  []okf.Problem
}

func NewEngine(client llm.Client) *Engine {
	return &Engine{
		Client:         client,
		SourceLoader:   source.NewLoader(),
		MaximumChunk:   DefaultChunkLimit,
		RequestTimeout: DefaultRequestWait,
		Now:            time.Now,
	}
}

// ExtractWorkspace normalizes sources before asking the LLM for validated
// concept JSON. It does not render Markdown.
func (engine *Engine) ExtractWorkspace(ctx context.Context, workspace project.Workspace, profile project.ProjectProfile) (ExtractionResult, error) {
	if engine == nil || engine.Client == nil {
		return ExtractionResult{}, fmt.Errorf("extract workspace %s: LLM client is required", workspace.Config.Name)
	}
	if err := project.ValidateProfile(profile); err != nil {
		return ExtractionResult{}, fmt.Errorf("extract workspace %s: invalid profile: %w", workspace.Config.Name, err)
	}
	loaded, err := engine.SourceLoader.LoadDirectory(ctx, workspace.InputDir)
	if err != nil {
		return ExtractionResult{}, fmt.Errorf("extract workspace %s: load sources: %w", workspace.Config.Name, err)
	}
	if len(loaded.Documents) == 0 {
		return ExtractionResult{}, fmt.Errorf("extract workspace %s: input directory contains no supported source documents", workspace.Config.Name)
	}
	documents, err := normalize.NormalizeDocuments(loaded.Documents)
	if err != nil {
		return ExtractionResult{}, fmt.Errorf("extract workspace %s: normalize sources: %w", workspace.Config.Name, err)
	}
	if err := normalize.WriteDocuments(workspace.NormalizedDir, documents); err != nil {
		return ExtractionResult{}, fmt.Errorf("extract workspace %s: persist normalized sources: %w", workspace.Config.Name, err)
	}

	timeout := engine.RequestTimeout
	if timeout <= 0 {
		timeout, err = profile.Extraction.Timeout()
		if err != nil {
			return ExtractionResult{}, fmt.Errorf("extract workspace %s: %w", workspace.Config.Name, err)
		}
	}
	maximumChunk := engine.MaximumChunk
	if maximumChunk <= 0 {
		maximumChunk = profile.Extraction.MaximumChunkCharacters
	}
	extractor, err := extract.NewExtractor(engine.Client, extract.Settings{
		MaximumChunkCharacters: maximumChunk,
		MaximumOutputConcepts:  profile.Extraction.MaximumConceptsPerSource,
		MaximumSourceExcerpts:  profile.Extraction.MaximumSourceExcerpts,
		MinimumConfidence:      profile.Extraction.MinimumConfidence,
		RequestTimeout:         timeout,
	})
	if err != nil {
		return ExtractionResult{}, fmt.Errorf("extract workspace %s: configure extractor: %w", workspace.Config.Name, err)
	}
	drafts, err := extractor.Extract(ctx, profile, documents)
	if err != nil {
		return ExtractionResult{}, fmt.Errorf("extract workspace %s: %w", workspace.Config.Name, err)
	}
	if err := writeJSON(workspace.ConceptDraftsPath, nonNilDrafts(drafts)); err != nil {
		return ExtractionResult{}, fmt.Errorf("extract workspace %s: persist concept drafts: %w", workspace.Config.Name, err)
	}
	return ExtractionResult{Documents: documents, Drafts: drafts, Warnings: loaded.Warnings}, nil
}

// GenerateWorkspace runs extraction, deterministic canonicalization,
// relationship resolution, rendering, and project validation.
func (engine *Engine) GenerateWorkspace(ctx context.Context, workspace project.Workspace, profile project.ProjectProfile) (GenerationResult, error) {
	extracted, err := engine.ExtractWorkspace(ctx, workspace, profile)
	if err != nil {
		return GenerationResult{}, err
	}
	concepts, err := canonicalize.Canonicalize(profile, extracted.Drafts)
	if err != nil {
		return GenerationResult{}, fmt.Errorf("generate workspace %s: canonicalize concepts: %w", workspace.Config.Name, err)
	}
	edges, unresolved := canonicalize.ResolveRelationships(profile, concepts)
	if err := writeJSON(workspace.CanonicalConceptsPath, nonNilConcepts(concepts)); err != nil {
		return GenerationResult{}, fmt.Errorf("generate workspace %s: persist canonical concepts: %w", workspace.Config.Name, err)
	}
	if err := writeJSON(workspace.UnresolvedRelationsPath, nonNilUnresolved(unresolved)); err != nil {
		return GenerationResult{}, fmt.Errorf("generate workspace %s: persist unresolved relations: %w", workspace.Config.Name, err)
	}

	generatedAt := time.Now()
	if engine != nil && engine.Now != nil {
		generatedAt = engine.Now()
	}
	warnings := sourceWarningStrings(extracted.Warnings)
	if len(unresolved) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d relationships could not be resolved; review staging/%s", len(unresolved), project.UnresolvedRelationsFilename))
	}
	renderConcepts := make([]okf.Concept, 0, len(concepts))
	for _, concept := range concepts {
		renderConcepts = append(renderConcepts, toOKFConcept(concept))
	}
	renderRelations := make([]okf.Relation, 0, len(edges))
	for _, edge := range edges {
		renderRelations = append(renderRelations, okf.Relation{
			From: edge.From, To: edge.To, Predicate: edge.Predicate, Confidence: edge.Confidence, Evidence: edge.Evidence,
		})
	}
	problems, err := okf.GenerateBundle(workspace.KnowledgeDir, profile, renderConcepts, renderRelations, okf.RunInfo{
		GeneratedAt:         generatedAt,
		Profile:             workspace.Config.Profile,
		SourceDocuments:     len(extracted.Documents),
		ExtractedDrafts:     len(extracted.Drafts),
		CanonicalConcepts:   len(concepts),
		UnresolvedRelations: len(unresolved),
		ValidationWarnings:  warnings,
		GeneratorVersion:    okf.GeneratorVersion,
	}, engine.StrictRelationships)
	result := GenerationResult{
		Documents: extracted.Documents, Drafts: extracted.Drafts, Concepts: concepts, Edges: edges,
		UnresolvedRelations: unresolved, SourceWarnings: extracted.Warnings, ValidationProblems: problems,
	}
	if err != nil {
		return result, fmt.Errorf("generate workspace %s: %w", workspace.Config.Name, err)
	}
	return result, nil
}

func LoadDrafts(filename string) ([]extract.ConceptDraft, error) {
	var drafts []extract.ConceptDraft
	if err := readStrictJSON(filename, &drafts); err != nil {
		return nil, fmt.Errorf("load concept drafts %s: %w", filename, err)
	}
	if drafts == nil {
		drafts = []extract.ConceptDraft{}
	}
	return drafts, nil
}

func LoadCanonicalConcepts(filename string) ([]canonicalize.CanonicalConcept, error) {
	var concepts []canonicalize.CanonicalConcept
	if err := readStrictJSON(filename, &concepts); err != nil {
		return nil, fmt.Errorf("load canonical concepts %s: %w", filename, err)
	}
	if concepts == nil {
		concepts = []canonicalize.CanonicalConcept{}
	}
	sort.Slice(concepts, func(i, j int) bool { return concepts[i].ID < concepts[j].ID })
	return concepts, nil
}

func LoadUnresolvedRelations(filename string) ([]canonicalize.UnresolvedRelation, error) {
	var relations []canonicalize.UnresolvedRelation
	if err := readStrictJSON(filename, &relations); err != nil {
		return nil, fmt.Errorf("load unresolved relations %s: %w", filename, err)
	}
	if relations == nil {
		relations = []canonicalize.UnresolvedRelation{}
	}
	return relations, nil
}

func writeJSON(filename string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON for %s: %w", filename, err)
	}
	data = append(data, '\n')
	if err := project.AtomicWriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("write JSON %s: %w", filename, err)
	}
	return nil
}

func readStrictJSON(filename string, output any) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("decode %s: %w", filename, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("decode %s: multiple JSON values", filename)
		}
		return fmt.Errorf("decode %s trailing content: %w", filename, err)
	}
	return nil
}

func toOKFConcept(concept canonicalize.CanonicalConcept) okf.Concept {
	attributes := make(map[string]any, len(concept.Attributes)+1)
	for key, value := range concept.Attributes {
		attributes[key] = value
	}
	if len(concept.Aliases) > 0 {
		if _, exists := attributes["aliases"]; !exists {
			attributes["aliases"] = append([]string(nil), concept.Aliases...)
		}
	}
	sections := make([]okf.Section, 0, len(concept.Sections))
	for _, section := range concept.Sections {
		sections = append(sections, okf.Section{Heading: section.Heading, Content: section.Content})
	}
	citations := make([]okf.Citation, 0, len(concept.Citations))
	for _, citation := range concept.Citations {
		citations = append(citations, okf.Citation{
			SourceID: citation.SourceID, URI: citation.URI, Page: citation.Page,
			StartLine: citation.StartLine, EndLine: citation.EndLine, Evidence: citation.Evidence,
		})
	}
	return okf.Concept{
		ID: concept.ID, Type: concept.Type, Title: concept.Title, Description: concept.Description,
		Resource: concept.Resource, Tags: append([]string(nil), concept.Tags...), Confidence: concept.Confidence,
		Attributes: attributes, Sections: sections, Citations: citations,
	}
}

func sourceWarningStrings(warnings []source.Warning) []string {
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		result = append(result, warning.String())
	}
	sort.Strings(result)
	return result
}

func nonNilDrafts(values []extract.ConceptDraft) []extract.ConceptDraft {
	if values == nil {
		return []extract.ConceptDraft{}
	}
	return values
}

func nonNilConcepts(values []canonicalize.CanonicalConcept) []canonicalize.CanonicalConcept {
	if values == nil {
		return []canonicalize.CanonicalConcept{}
	}
	return values
}

func nonNilUnresolved(values []canonicalize.UnresolvedRelation) []canonicalize.UnresolvedRelation {
	if values == nil {
		return []canonicalize.UnresolvedRelation{}
	}
	return values
}

func joinedSafetyInstructions(profile project.ProjectProfile) string {
	var lines []string
	for _, instruction := range profile.OKF.SafetyInstructions {
		if instruction = strings.TrimSpace(instruction); instruction != "" {
			lines = append(lines, instruction)
		}
	}
	return strings.Join(lines, "\n")
}
