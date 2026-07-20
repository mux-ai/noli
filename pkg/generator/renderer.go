package generator

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"noli/pkg/okf"
)

// predicatePhrases maps recognized predicates to the deterministic Markdown
// phrases the parser normalizes back (docs/PROTOCOL.md section 7). Unknown
// predicates are rendered verbatim and reparse as "links-to".
var predicatePhrases = map[string]string{
	"applies-to":  "Applies to",
	"enforced-by": "Enforced by",
	"depends-on":  "Depends on",
	"uses":        "Uses",
	"follows":     "Follows",
}

// renderConcept renders one concept document deterministically. Timestamps
// appear only when the input supplied one.
func renderConcept(concept resolvedConcept, titles map[string]string) ([]byte, error) {
	input := concept.Input
	extra := make(map[string]any, len(input.Attributes))
	for key, value := range input.Attributes {
		extra[key] = value
	}
	metadata := okf.Metadata{
		Type:        concept.Type,
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		Resource:    strings.TrimSpace(input.Resource),
		Tags:        normalizedTags(input.Tags),
		Timestamp:   strings.TrimSpace(input.Timestamp),
		Extra:       extra,
	}
	frontmatter, err := renderFrontmatter(metadata)
	if err != nil {
		return nil, fmt.Errorf("render concept %q: %w", concept.ID, err)
	}

	var body strings.Builder
	for _, section := range input.Sections {
		heading := cleanHeading(section.Heading)
		if heading == "" {
			return nil, fmt.Errorf("render concept %q: section heading is required", concept.ID)
		}
		writeSection(&body, heading, section.Content)
	}
	if related := renderRelationships(concept, titles); related != "" {
		writeSection(&body, "Relationships", related)
	}
	if citations := renderCitations(input.Citations); citations != "" {
		writeSection(&body, "Citations", citations)
	}
	return append(frontmatter, []byte(body.String())...), nil
}

func renderFrontmatter(metadata okf.Metadata) ([]byte, error) {
	var yamlBuffer bytes.Buffer
	encoder := yaml.NewEncoder(&yamlBuffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(metadata); err != nil {
		return nil, fmt.Errorf("encode frontmatter: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close frontmatter encoder: %w", err)
	}
	var result bytes.Buffer
	result.WriteString("---\n")
	result.Write(yamlBuffer.Bytes())
	result.WriteString("---\n\n")
	return result.Bytes(), nil
}

func writeSection(builder *strings.Builder, heading, content string) {
	if builder.Len() > 0 {
		builder.WriteByte('\n')
	}
	builder.WriteString("## ")
	builder.WriteString(cleanHeading(heading))
	builder.WriteString("\n")
	content = strings.TrimSpace(content)
	if content != "" {
		builder.WriteString("\n")
		builder.WriteString(content)
		builder.WriteString("\n")
	}
}

func renderRelationships(concept resolvedConcept, titles map[string]string) string {
	var lines []string
	seen := make(map[resolvedRelationship]struct{})
	for _, relationship := range concept.Relationships {
		if _, duplicate := seen[relationship]; duplicate {
			continue
		}
		seen[relationship] = struct{}{}
		phrase, known := predicatePhrases[relationship.Predicate]
		if !known {
			phrase = strings.TrimSpace(relationship.Predicate)
		}
		label := strings.TrimSpace(titles[relationship.TargetID])
		if label == "" {
			label = filepath.Base(relationship.TargetID)
		}
		lines = append(lines, "- "+phrase+": ["+escapeLinkLabel(label)+"]("+
			bundleLink(relationship.TargetID)+")")
	}
	return strings.Join(lines, "\n")
}

// bundleLink builds an absolute bundle-relative Markdown link. OKF v0.1
// section 5.1 recommends this form because it stays valid when documents
// move within their subdirectory.
func bundleLink(toID string) string {
	return "/" + toID + ".md"
}

func renderCitations(citations []CitationInput) string {
	items := append([]CitationInput(nil), citations...)
	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if left.Source != right.Source {
			return left.Source < right.Source
		}
		if left.URI != right.URI {
			return left.URI < right.URI
		}
		if left.Page != right.Page {
			return left.Page < right.Page
		}
		if left.StartLine != right.StartLine {
			return left.StartLine < right.StartLine
		}
		return left.Evidence < right.Evidence
	})
	seen := make(map[CitationInput]struct{})
	var lines []string
	for _, citation := range items {
		if _, duplicate := seen[citation]; duplicate {
			continue
		}
		seen[citation] = struct{}{}
		label := strings.TrimSpace(citation.Source)
		if label == "" {
			label = strings.TrimSpace(citation.URI)
		}
		if label == "" {
			label = "Source"
		}
		line := "[" + strconv.Itoa(len(lines)+1) + "] "
		if uri := strings.TrimSpace(citation.URI); uri != "" {
			line += "[" + escapeLinkLabel(label) + "](" + escapeLinkDestination(uri) + ")"
		} else {
			line += escapeInline(label)
		}
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
		if !strings.HasSuffix(line, ".") && !strings.HasSuffix(line, "!") && !strings.HasSuffix(line, "?") {
			line += "."
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// renderDirectoryIndex renders a directory index.md. OKF v0.1 section 6
// requires index files to carry no frontmatter and to group concepts under
// headings as `* [Title](link) - description` bullets.
func renderDirectoryIndex(directory, typeName string, entries []resolvedConcept) ([]byte, error) {
	if strings.TrimSpace(typeName) == "" {
		return nil, fmt.Errorf("render index for %q: concept type is required", directory)
	}
	sorted := append([]resolvedConcept(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	var body strings.Builder
	body.WriteString("# ")
	body.WriteString(cleanHeading(typeName))
	body.WriteString("\n\n")
	for _, entry := range sorted {
		body.WriteString("* [")
		body.WriteString(escapeLinkLabel(strings.TrimSpace(entry.Input.Title)))
		body.WriteString("](")
		body.WriteString(bundleLink(entry.ID))
		body.WriteString(")")
		if description := strings.TrimSpace(entry.Input.Description); description != "" {
			body.WriteString(" - ")
			body.WriteString(escapeInline(description))
		}
		body.WriteString("\n")
	}
	return []byte(body.String()), nil
}

// renderRootIndex renders the bundle root index.md, also frontmatter-free
// per OKF v0.1 section 6.
func renderRootIndex(projectName string, directories map[string]string, counts map[string]int) ([]byte, error) {
	names := make([]string, 0, len(directories))
	for directory := range directories {
		names = append(names, directory)
	}
	sort.Strings(names)
	var body strings.Builder
	body.WriteString("# ")
	body.WriteString(cleanHeading(projectName))
	body.WriteString("\n\n## Knowledge directories\n\n")
	for _, directory := range names {
		body.WriteString("* [")
		body.WriteString(escapeLinkLabel(directories[directory]))
		body.WriteString("](")
		body.WriteString(bundleLink(directory + "/index"))
		body.WriteString(") - ")
		body.WriteString(strconv.Itoa(counts[directory]))
		body.WriteString(" documents\n")
	}
	return []byte(body.String()), nil
}

func cleanHeading(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "))
	value = strings.TrimLeft(value, "#")
	return strings.TrimSpace(value)
}

func escapeLinkLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "[", "\\[")
	return strings.ReplaceAll(value, "]", "\\]")
}

// escapeLinkDestination keeps a citation URI inside one Markdown link
// destination without changing its meaning. Angle brackets allow URL query
// strings to contain parentheses; literal brackets are percent-encoded.
func escapeLinkDestination(value string) string {
	value = escapeInline(value)
	value = strings.ReplaceAll(value, "<", "%3C")
	value = strings.ReplaceAll(value, ">", "%3E")
	return "<" + value + ">"
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
