package okf

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

type RenderOptions struct {
	Timestamp            string
	LinkStyle            string
	IncludeEmptySections bool
	SectionOrder         map[string][]string
}

// RenderConcept renders one concept as deterministic Markdown with YAML
// frontmatter. targetTitles maps canonical IDs to display labels.
func RenderConcept(concept Concept, relations []Relation, targetTitles map[string]string, options RenderOptions) ([]byte, error) {
	if err := ValidateConceptID(concept.ID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(concept.Type) == "" {
		return nil, errors.New("render concept: type is required")
	}
	if strings.TrimSpace(concept.Title) == "" {
		return nil, errors.New("render concept: title is required")
	}

	extra := make(map[string]any, len(concept.Attributes)+1)
	for key, value := range concept.Attributes {
		if isReservedMetadataKey(key) || strings.EqualFold(key, "confidence") {
			return nil, fmt.Errorf("render concept %q: attribute %q conflicts with a reserved metadata field", concept.ID, key)
		}
		extra[key] = value
	}
	extra["confidence"] = concept.Confidence
	timestamp := strings.TrimSpace(concept.Timestamp)
	if timestamp == "" {
		timestamp = strings.TrimSpace(options.Timestamp)
	}
	metadata := Metadata{
		Type:        strings.TrimSpace(concept.Type),
		Title:       strings.TrimSpace(concept.Title),
		Description: strings.TrimSpace(concept.Description),
		Resource:    strings.TrimSpace(concept.Resource),
		Tags:        normalizedTags(concept.Tags),
		Timestamp:   timestamp,
		Extra:       extra,
	}
	frontmatter, err := renderFrontmatter(metadata)
	if err != nil {
		return nil, fmt.Errorf("render concept %q frontmatter: %w", concept.ID, err)
	}

	sections := normalizeSections(concept.Sections, options.SectionOrder[concept.Type], options.IncludeEmptySections)
	var body strings.Builder
	for _, section := range sections {
		writeSection(&body, section.Heading, section.Content)
	}
	related := renderRelations(concept.ID, relations, targetTitles, options.LinkStyle)
	if related != "" {
		writeSection(&body, "Related knowledge", related)
	}
	citations := renderCitations(concept.Citations)
	if citations != "" {
		writeSection(&body, "Citations", citations)
	}

	return append(frontmatter, []byte(body.String())...), nil
}

func renderFrontmatter(metadata Metadata) ([]byte, error) {
	var yamlBuffer bytes.Buffer
	encoder := yaml.NewEncoder(&yamlBuffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(metadata); err != nil {
		return nil, fmt.Errorf("encode YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close YAML encoder: %w", err)
	}
	var result bytes.Buffer
	result.WriteString("---\n")
	result.Write(yamlBuffer.Bytes())
	result.WriteString("---\n")
	if len(metadata.Extra) > 0 || metadata.Title != "" || metadata.Type != "" {
		result.WriteByte('\n')
	}
	return result.Bytes(), nil
}

func normalizeSections(input []Section, preferred []string, includeEmpty bool) []Section {
	contents := make(map[string][]string)
	labels := make(map[string]string)
	for _, section := range input {
		heading := cleanHeading(section.Heading)
		if heading == "" {
			continue
		}
		key := strings.ToLower(heading)
		content := strings.TrimSpace(section.Content)
		if content == "" && !includeEmpty {
			continue
		}
		if _, exists := labels[key]; !exists {
			labels[key] = heading
		}
		duplicate := false
		for _, current := range contents[key] {
			if current == content {
				duplicate = true
				break
			}
		}
		if !duplicate {
			contents[key] = append(contents[key], content)
		}
	}

	var keys []string
	added := make(map[string]struct{})
	for _, heading := range preferred {
		key := strings.ToLower(cleanHeading(heading))
		if _, exists := contents[key]; exists {
			keys = append(keys, key)
			added[key] = struct{}{}
		}
	}
	var remaining []string
	for key := range contents {
		if _, exists := added[key]; !exists && key != "related knowledge" && key != "citations" {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	keys = append(keys, remaining...)
	result := make([]Section, 0, len(keys))
	for _, key := range keys {
		result = append(result, Section{Heading: labels[key], Content: strings.Join(contents[key], "\n\n")})
	}
	return result
}

func writeSection(builder *strings.Builder, heading, content string) {
	if builder.Len() > 0 {
		builder.WriteByte('\n')
	}
	builder.WriteString("# ")
	builder.WriteString(cleanHeading(heading))
	builder.WriteString("\n\n")
	content = strings.TrimSpace(content)
	if content != "" {
		builder.WriteString(content)
		builder.WriteByte('\n')
	}
}

func cleanHeading(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "))
	value = strings.TrimLeft(value, "#")
	return strings.TrimSpace(value)
}

func renderRelations(from string, relations []Relation, titles map[string]string, style string) string {
	filtered := make([]Relation, 0, len(relations))
	seen := make(map[string]struct{})
	for _, relation := range relations {
		if relation.From != from || ValidateConceptID(relation.To) != nil {
			continue
		}
		key := relation.Predicate + "\x00" + relation.To
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, relation)
	}
	sort.Slice(filtered, func(i, j int) bool {
		left, right := strings.ToLower(filtered[i].Predicate), strings.ToLower(filtered[j].Predicate)
		if left != right {
			return left < right
		}
		return filtered[i].To < filtered[j].To
	})
	var lines []string
	for _, relation := range filtered {
		label := strings.TrimSpace(titles[relation.To])
		if label == "" {
			label = filepath.Base(relation.To)
		}
		predicate := strings.TrimSpace(relation.Predicate)
		if predicate == "" {
			predicate = "Related to"
		}
		link := markdownLink(from, relation.To, style)
		lines = append(lines, "- "+escapeInline(predicate)+" ["+escapeLinkLabel(label)+"]("+link+").")
	}
	return strings.Join(lines, "\n")
}

func markdownLink(from, to, style string) string {
	if style == "relative" {
		fromDirectory := filepath.Dir(filepath.FromSlash(from + ".md"))
		target := filepath.FromSlash(to + ".md")
		relative, err := filepath.Rel(fromDirectory, target)
		if err == nil {
			return filepath.ToSlash(relative)
		}
	}
	return "/" + filepath.ToSlash(to) + ".md"
}

func renderCitations(citations []Citation) string {
	items := append([]Citation(nil), citations...)
	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if left.SourceID != right.SourceID {
			return left.SourceID < right.SourceID
		}
		if left.Page != right.Page {
			return left.Page < right.Page
		}
		if left.StartLine != right.StartLine {
			return left.StartLine < right.StartLine
		}
		if left.EndLine != right.EndLine {
			return left.EndLine < right.EndLine
		}
		return left.Evidence < right.Evidence
	})
	seen := make(map[string]struct{})
	var lines []string
	for _, citation := range items {
		key := fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%d\x00%s", citation.SourceID, citation.URI, citation.Page, citation.StartLine, citation.EndLine, citation.Evidence)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		label := strings.TrimSpace(citation.SourceID)
		if label == "" {
			label = strings.TrimSpace(citation.URI)
		}
		if label == "" {
			label = "Source"
		}
		line := "- " + escapeInline(label)
		var locations []string
		if citation.Page > 0 {
			locations = append(locations, "page "+strconv.Itoa(citation.Page))
		}
		if citation.StartLine > 0 {
			lineRange := "line " + strconv.Itoa(citation.StartLine)
			if citation.EndLine > citation.StartLine {
				lineRange = "lines " + strconv.Itoa(citation.StartLine) + "-" + strconv.Itoa(citation.EndLine)
			}
			locations = append(locations, lineRange)
		}
		if len(locations) > 0 {
			line += ", " + strings.Join(locations, ", ")
		}
		if evidence := strings.TrimSpace(citation.Evidence); evidence != "" {
			line += ": " + escapeInline(evidence)
		}
		lines = append(lines, line+".")
	}
	return strings.Join(lines, "\n")
}

func escapeLinkLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "[", "\\[")
	return strings.ReplaceAll(value, "]", "\\]")
}

func escapeInline(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	return strings.TrimSpace(value)
}

func normalizedTags(tags []string) []string {
	byComparison := make(map[string]string)
	for _, tag := range tags {
		tag = strings.Join(strings.Fields(tag), " ")
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if current, exists := byComparison[key]; !exists || tag < current {
			byComparison[key] = tag
		}
	}
	keys := make([]string, 0, len(byComparison))
	for key := range byComparison {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, len(keys))
	for i, key := range keys {
		result[i] = byComparison[key]
	}
	return result
}

// ValidateConceptID rejects IDs that could escape or alias paths.
func ValidateConceptID(id string) error {
	if id == "" || filepath.IsAbs(id) || strings.HasPrefix(id, "/") || strings.Contains(id, "\\") {
		return fmt.Errorf("unsafe concept ID %q", id)
	}
	clean := filepath.Clean(filepath.FromSlash(id))
	if clean == "." || clean == ".." || clean != filepath.FromSlash(id) || escapesRoot(clean) {
		return fmt.Errorf("unsafe concept ID %q", id)
	}
	for _, part := range strings.Split(filepath.ToSlash(clean), "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("unsafe concept ID %q", id)
		}
	}
	return nil
}

func conceptOutputPath(root, id string) (string, error) {
	if err := ValidateConceptID(id); err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve output root %q: %w", root, err)
	}
	target := filepath.Join(absRoot, filepath.FromSlash(id+".md"))
	rel, err := filepath.Rel(absRoot, target)
	if err != nil || escapesRoot(rel) {
		return "", fmt.Errorf("concept output path for %q escapes root", id)
	}
	return target, nil
}

// WriteFileAtomic writes a file through a temporary sibling and rename.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create directory %q: %w", directory, err)
	}
	temporary, err := os.CreateTemp(directory, ".noli-write-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %q: %w", path, err)
	}
	temporaryName := temporary.Name()
	cleaned := false
	defer func() {
		if !cleaned {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set mode on temporary file for %q: %w", path, err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary file for %q: %w", path, err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary file for %q: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file for %q: %w", path, err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace file %q: %w", path, err)
	}
	cleaned = true
	return nil
}

func formatTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
