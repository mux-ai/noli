package canonicalize

import "noli/internal/extract"

type CanonicalConcept struct {
	ID             string                  `json:"permanent_id"`
	Directory      string                  `json:"directory"`
	Type           string                  `json:"type"`
	Title          string                  `json:"title"`
	Description    string                  `json:"description"`
	Resource       string                  `json:"resource,omitempty"`
	Tags           []string                `json:"tags,omitempty"`
	Attributes     map[string]any          `json:"attributes,omitempty"`
	Sections       []extract.SectionDraft  `json:"sections"`
	Relations      []extract.RelationDraft `json:"relations,omitempty"`
	Citations      []extract.Citation      `json:"citations,omitempty"`
	Confidence     float64                 `json:"confidence"`
	Aliases        []string                `json:"aliases,omitempty"`
	SourceDraftIDs []string                `json:"source_draft_ids,omitempty"`
	Conflicts      map[string][]any        `json:"conflicts,omitempty"`
}

type DuplicateGroup struct {
	Key          string   `json:"key"`
	TemporaryIDs []string `json:"temporary_ids"`
}
