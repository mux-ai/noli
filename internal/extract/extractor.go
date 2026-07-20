package extract

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"noli/internal/llm"
	"noli/internal/project"
	"noli/internal/source"
)

const (
	DefaultMaximumChunkCharacters = 12000
	DefaultMaximumOutputConcepts  = 30
	DefaultMaximumSourceExcerpts  = 4
	DefaultRequestTimeout         = 2 * time.Minute
)

type Settings struct {
	MaximumChunkCharacters int
	MaximumOutputConcepts  int
	MaximumSourceExcerpts  int
	MinimumConfidence      float64
	RequestTimeout         time.Duration
}

type Extractor struct {
	client   llm.Client
	settings Settings
}

type SourceExcerpt struct {
	SourceID  string `json:"source_id"`
	Name      string `json:"name"`
	URI       string `json:"uri,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Heading   string `json:"heading,omitempty"`
	Content   string `json:"content"`
	Page      int    `json:"page,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

func NewExtractor(client llm.Client, settings Settings) (*Extractor, error) {
	if client == nil {
		return nil, fmt.Errorf("create extractor: LLM client is required")
	}
	if settings.MaximumChunkCharacters <= 0 {
		settings.MaximumChunkCharacters = DefaultMaximumChunkCharacters
	}
	if settings.MaximumOutputConcepts <= 0 {
		settings.MaximumOutputConcepts = DefaultMaximumOutputConcepts
	}
	if settings.MaximumSourceExcerpts <= 0 {
		settings.MaximumSourceExcerpts = DefaultMaximumSourceExcerpts
	}
	if settings.RequestTimeout <= 0 {
		settings.RequestTimeout = DefaultRequestTimeout
	}
	if settings.MinimumConfidence < 0 || settings.MinimumConfidence > 1 {
		return nil, fmt.Errorf("create extractor: minimum confidence must be between 0 and 1")
	}
	return &Extractor{client: client, settings: settings}, nil
}

func (e *Extractor) Extract(ctx context.Context, profile project.ProjectProfile, documents []source.SourceDocument) ([]ConceptDraft, error) {
	if e == nil || e.client == nil {
		return nil, fmt.Errorf("extract concepts: extractor is not initialized")
	}
	maximumChunkCharacters := e.settings.MaximumChunkCharacters
	if profile.Extraction.MaximumChunkCharacters > 0 && profile.Extraction.MaximumChunkCharacters < maximumChunkCharacters {
		maximumChunkCharacters = profile.Extraction.MaximumChunkCharacters
	}
	maximumSourceExcerpts := e.settings.MaximumSourceExcerpts
	if profile.Extraction.MaximumSourceExcerpts > 0 && profile.Extraction.MaximumSourceExcerpts < maximumSourceExcerpts {
		maximumSourceExcerpts = profile.Extraction.MaximumSourceExcerpts
	}
	requestTimeout := e.settings.RequestTimeout
	if profileTimeout, err := profile.Extraction.Timeout(); err != nil {
		return nil, fmt.Errorf("extract concepts: %w", err)
	} else if profileTimeout > 0 && profileTimeout < requestTimeout {
		requestTimeout = profileTimeout
	}
	excerpts := buildExcerpts(documents, maximumChunkCharacters)
	batches := batchExcerpts(excerpts, maximumSourceExcerpts, maximumChunkCharacters)
	minimumConfidence := e.settings.MinimumConfidence
	if profile.Extraction.MinimumConfidence > minimumConfidence {
		minimumConfidence = profile.Extraction.MinimumConfidence
	}
	perSourceLimit := profile.Extraction.MaximumConceptsPerSource
	if perSourceLimit <= 0 {
		perSourceLimit = e.settings.MaximumOutputConcepts
	}
	perSourceCounts := make(map[string]int)
	result := make([]ConceptDraft, 0)
	for batchIndex, batch := range batches {
		prompt, err := BuildExtractionPrompt(profile, batch, e.settings.MaximumOutputConcepts)
		if err != nil {
			return nil, fmt.Errorf("extract concepts batch %d: %w", batchIndex+1, err)
		}
		var response ConceptResponse
		requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		err = e.client.GenerateStructured(requestCtx, ExtractionSystemPrompt, prompt, &response)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("extract concepts batch %d: %w", batchIndex+1, err)
		}
		if response.Concepts == nil {
			return nil, fmt.Errorf("extract concepts batch %d: response omitted required concepts field", batchIndex+1)
		}
		if len(response.Concepts) > e.settings.MaximumOutputConcepts {
			return nil, fmt.Errorf("extract concepts batch %d: response has %d concepts, limit is %d", batchIndex+1, len(response.Concepts), e.settings.MaximumOutputConcepts)
		}
		batchSourceIDs := make(map[string]struct{}, len(batch))
		for _, excerpt := range batch {
			batchSourceIDs[excerpt.SourceID] = struct{}{}
		}
		if err := ValidateConcepts(response.Concepts, profile, batchSourceIDs); err != nil {
			return nil, fmt.Errorf("extract concepts batch %d: %w", batchIndex+1, err)
		}
		for conceptIndex := range response.Concepts {
			if conceptType, exists := profile.ConceptType(response.Concepts[conceptIndex].Type); exists {
				response.Concepts[conceptIndex].Type = conceptType.Type
			}
		}
		for _, concept := range response.Concepts {
			if concept.Confidence < minimumConfidence {
				continue
			}
			primarySource := conceptPrimarySource(concept, batch)
			if primarySource != "" && perSourceCounts[primarySource] >= perSourceLimit {
				continue
			}
			if primarySource != "" {
				perSourceCounts[primarySource]++
			}
			result = append(result, concept)
		}
	}
	ensureUniqueTemporaryIDs(result)
	return result, nil
}

func buildExcerpts(documents []source.SourceDocument, maximumCharacters int) []SourceExcerpt {
	sorted := append([]source.SourceDocument(nil), documents...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].ID != sorted[j].ID {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].SourceURI < sorted[j].SourceURI
	})
	result := make([]SourceExcerpt, 0)
	for _, document := range sorted {
		if len(document.Sections) == 0 {
			for _, part := range splitText(document.Content, maximumCharacters) {
				result = append(result, SourceExcerpt{SourceID: document.ID, Name: document.Name, URI: document.SourceURI, MediaType: document.MediaType, Content: part})
			}
			continue
		}
		for _, section := range document.Sections {
			parts := splitText(section.Content, maximumCharacters)
			for _, part := range parts {
				result = append(result, SourceExcerpt{
					SourceID: document.ID, Name: document.Name, URI: document.SourceURI, MediaType: document.MediaType,
					Heading: section.Heading, Content: part, Page: section.Page, StartLine: section.StartLine, EndLine: section.EndLine,
				})
			}
		}
	}
	return result
}

func splitText(value string, maximumCharacters int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if utf8.RuneCountInString(value) <= maximumCharacters {
		return []string{value}
	}
	paragraphs := strings.Split(value, "\n\n")
	result := make([]string, 0)
	var current strings.Builder
	flush := func() {
		trimmed := strings.TrimSpace(current.String())
		if trimmed != "" {
			result = append(result, trimmed)
		}
		current.Reset()
	}
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		if utf8.RuneCountInString(paragraph) > maximumCharacters {
			flush()
			runes := []rune(paragraph)
			for len(runes) > 0 {
				end := maximumCharacters
				if end > len(runes) {
					end = len(runes)
				}
				result = append(result, strings.TrimSpace(string(runes[:end])))
				runes = runes[end:]
			}
			continue
		}
		separator := 0
		if current.Len() > 0 {
			separator = 2
		}
		if utf8.RuneCountInString(current.String())+separator+utf8.RuneCountInString(paragraph) > maximumCharacters {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(paragraph)
	}
	flush()
	return result
}

func batchExcerpts(excerpts []SourceExcerpt, maximumExcerpts, maximumCharacters int) [][]SourceExcerpt {
	result := make([][]SourceExcerpt, 0)
	current := make([]SourceExcerpt, 0, maximumExcerpts)
	characters := 0
	for _, excerpt := range excerpts {
		count := utf8.RuneCountInString(excerpt.Content)
		if len(current) > 0 && (len(current) >= maximumExcerpts || characters+count > maximumCharacters) {
			result = append(result, current)
			current = make([]SourceExcerpt, 0, maximumExcerpts)
			characters = 0
		}
		current = append(current, excerpt)
		characters += count
	}
	if len(current) > 0 {
		result = append(result, current)
	}
	return result
}

func conceptPrimarySource(concept ConceptDraft, excerpts []SourceExcerpt) string {
	if len(concept.Citations) > 0 {
		return concept.Citations[0].SourceID
	}
	if len(excerpts) > 0 {
		return excerpts[0].SourceID
	}
	return ""
}

func ensureUniqueTemporaryIDs(concepts []ConceptDraft) {
	used := make(map[string]int)
	for i := range concepts {
		base := strings.TrimSpace(concepts[i].TemporaryID)
		used[base]++
		if used[base] > 1 {
			concepts[i].TemporaryID = fmt.Sprintf("%s-%d", base, used[base])
		}
	}
}
