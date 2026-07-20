package okf

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RunInfo struct {
	GeneratedAt         time.Time
	Profile             string
	SourceDocuments     int
	ExtractedDrafts     int
	CanonicalConcepts   int
	UnresolvedRelations int
	ValidationWarnings  []string
	GeneratorVersion    string
}

func RenderLog(run RunInfo) ([]byte, error) {
	timestamp := formatTimestamp(run.GeneratedAt)
	metadata := Metadata{Type: "Bundle Log", Title: "Generation log", Timestamp: timestamp}
	frontmatter, err := renderFrontmatter(metadata)
	if err != nil {
		return nil, fmt.Errorf("render bundle log frontmatter: %w", err)
	}
	var body strings.Builder
	body.WriteString("# Generation log\n\n")
	fields := []struct {
		name  string
		value string
	}{
		{"Generation timestamp", timestamp},
		{"Project profile", strings.TrimSpace(run.Profile)},
		{"Source document count", strconv.Itoa(run.SourceDocuments)},
		{"Extracted draft count", strconv.Itoa(run.ExtractedDrafts)},
		{"Canonical concept count", strconv.Itoa(run.CanonicalConcepts)},
		{"Unresolved relation count", strconv.Itoa(run.UnresolvedRelations)},
		{"Generator version", strings.TrimSpace(run.GeneratorVersion)},
	}
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		body.WriteString("- ")
		body.WriteString(field.name)
		body.WriteString(": ")
		body.WriteString(escapeInline(field.value))
		body.WriteString("\n")
	}
	warnings := append([]string(nil), run.ValidationWarnings...)
	sort.Strings(warnings)
	if len(warnings) > 0 {
		body.WriteString("\n## Validation warnings\n\n")
		for _, warning := range warnings {
			if warning = strings.TrimSpace(warning); warning != "" {
				body.WriteString("- ")
				body.WriteString(escapeInline(warning))
				body.WriteString("\n")
			}
		}
	}
	return append(frontmatter, []byte(body.String())...), nil
}
