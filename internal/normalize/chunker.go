package normalize

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type Chunk struct {
	ID        string         `json:"id"`
	SourceID  string         `json:"source_id"`
	Name      string         `json:"name"`
	SourceURI string         `json:"source_uri"`
	MediaType string         `json:"media_type"`
	Content   string         `json:"content"`
	Sections  []Section      `json:"sections,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type Chunker struct {
	MaximumCharacters int
}

func (c Chunker) Chunk(document Document) ([]Chunk, error) {
	if c.MaximumCharacters < 1 {
		return nil, fmt.Errorf("chunk source %s: maximum characters must be at least 1", document.ID)
	}
	normalized, err := NormalizeDocument(document)
	if err != nil {
		return nil, fmt.Errorf("chunk source %s: %w", document.ID, err)
	}
	units := chunkUnits(normalized)
	var chunks []Chunk
	var contentParts []string
	var sections []Section
	contentLength := 0
	flush := func() {
		if len(contentParts) == 0 {
			return
		}
		chunks = append(chunks, Chunk{
			SourceID:  normalized.ID,
			Name:      normalized.Name,
			SourceURI: normalized.SourceURI,
			MediaType: normalized.MediaType,
			Content:   strings.Join(contentParts, "\n\n"),
			Sections:  append([]Section(nil), sections...),
			Metadata:  normalized.Metadata,
		})
		contentParts = nil
		sections = nil
		contentLength = 0
	}
	for _, unit := range units {
		parts := splitAtCharacterLimit(unit.text, c.MaximumCharacters)
		for partIndex, part := range parts {
			partLength := len([]rune(part))
			separatorLength := 0
			if len(contentParts) > 0 {
				separatorLength = 2
			}
			if contentLength+separatorLength+partLength > c.MaximumCharacters {
				flush()
				separatorLength = 0
			}
			section := unit.section
			section.Content = part
			if partIndex > 0 && strings.HasPrefix(part, "#") {
				section.Heading = ""
			}
			contentParts = append(contentParts, part)
			sections = append(sections, section)
			contentLength += separatorLength + partLength
		}
	}
	flush()
	for i := range chunks {
		chunks[i].ID = fmt.Sprintf("%s#chunk-%04d", normalized.ID, i+1)
	}
	return chunks, nil
}

func ChunkDocuments(documents []Document, maximumCharacters int) ([]Chunk, error) {
	normalized, err := NormalizeDocuments(documents)
	if err != nil {
		return nil, err
	}
	var chunks []Chunk
	chunker := Chunker{MaximumCharacters: maximumCharacters}
	for _, document := range normalized {
		documentChunks, err := chunker.Chunk(document)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, documentChunks...)
	}
	sort.SliceStable(chunks, func(i, j int) bool { return chunks[i].ID < chunks[j].ID })
	return chunks, nil
}

type chunkUnit struct {
	text    string
	section Section
}

func chunkUnits(document Document) []chunkUnit {
	if len(document.Sections) == 0 {
		if document.Content == "" {
			return nil
		}
		return []chunkUnit{{text: document.Content, section: Section{Content: document.Content}}}
	}
	units := make([]chunkUnit, 0, len(document.Sections))
	for _, section := range document.Sections {
		text := section.Content
		if section.Heading != "" {
			text = "# " + section.Heading
			if section.Content != "" {
				text += "\n\n" + section.Content
			}
		}
		if text == "" {
			continue
		}
		units = append(units, chunkUnit{text: text, section: section})
	}
	return units
}

func splitAtCharacterLimit(value string, maximum int) []string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return nil
	}
	parts := make([]string, 0, (len(runes)+maximum-1)/maximum)
	for len(runes) > maximum {
		cut := maximum
		minimumUsefulCut := maximum / 2
		for i := maximum; i > minimumUsefulCut; i-- {
			if runes[i-1] == '\n' {
				cut = i
				break
			}
			if unicode.IsSpace(runes[i-1]) && cut == maximum {
				cut = i
			}
		}
		part := strings.TrimSpace(string(runes[:cut]))
		if part != "" {
			parts = append(parts, part)
		}
		runes = []rune(strings.TrimSpace(string(runes[cut:])))
	}
	if len(runes) > 0 {
		parts = append(parts, string(runes))
	}
	return parts
}
