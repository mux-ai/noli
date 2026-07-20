package extract

type ConceptDraft struct {
	TemporaryID string          `json:"temporary_id"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Resource    string          `json:"resource,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Attributes  map[string]any  `json:"attributes,omitempty"`
	Sections    []SectionDraft  `json:"sections"`
	Relations   []RelationDraft `json:"relations,omitempty"`
	Citations   []Citation      `json:"citations,omitempty"`
	Confidence  float64         `json:"confidence"`
}

type SectionDraft struct {
	Heading string `json:"heading"`
	Content string `json:"content"`
}

type Citation struct {
	SourceID  string `json:"source_id"`
	URI       string `json:"uri,omitempty"`
	Page      int    `json:"page,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Evidence  string `json:"evidence,omitempty"`
}
