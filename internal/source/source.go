package source

import (
	"context"
	"fmt"
)

type SourceDocument struct {
	ID        string          `json:"id" yaml:"id"`
	Name      string          `json:"name" yaml:"name"`
	SourceURI string          `json:"source_uri" yaml:"source_uri"`
	MediaType string          `json:"media_type" yaml:"media_type"`
	Content   string          `json:"content" yaml:"content"`
	Sections  []SourceSection `json:"sections,omitempty" yaml:"sections,omitempty"`
	Metadata  map[string]any  `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type SourceSection struct {
	Heading   string `json:"heading,omitempty" yaml:"heading,omitempty"`
	Content   string `json:"content" yaml:"content"`
	Page      int    `json:"page,omitempty" yaml:"page,omitempty"`
	StartLine int    `json:"start_line,omitempty" yaml:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty" yaml:"end_line,omitempty"`
}

type File struct {
	ID        string
	Name      string
	Path      string
	SourceURI string
	MediaType string
}

// Adapter converts one safely resolved local file into the common source
// model. PDFAdapter is also usable independently when callers need to probe
// or configure pdftotext explicitly.
type Adapter interface {
	Load(ctx context.Context, file File) (SourceDocument, error)
}

type Warning struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (w Warning) String() string {
	if w.Path == "" {
		return w.Message
	}
	return fmt.Sprintf("%s: %s", w.Path, w.Message)
}

type LoadResult struct {
	Documents []SourceDocument `json:"documents"`
	Warnings  []Warning        `json:"warnings,omitempty"`
}
