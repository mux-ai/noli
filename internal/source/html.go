package source

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"
)

type htmlAdapter struct{}

func (htmlAdapter) Load(_ context.Context, file File) (SourceDocument, error) {
	raw, err := readUTF8File(file.Path)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load HTML source %s: %w", file.Path, err)
	}
	content, title := readableHTML(raw)
	metadata := make(map[string]any)
	if title != "" {
		metadata["html_title"] = title
	}
	return newDocument(file, content, markdownSections(content), metadata), nil
}

// readableHTML is deliberately conservative. It extracts visible text using
// only the standard library, drops active/non-visible elements, and represents
// HTML headings as Markdown headings so normal section processing applies.
func readableHTML(raw string) (string, string) {
	commentPattern := regexp.MustCompile(`(?is)<!--.*?-->`)
	scriptPattern := regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>`)
	stylePattern := regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style\s*>`)
	titlePattern := regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title\s*>`)
	tagPattern := regexp.MustCompile(`(?is)<[^>]+>`)
	headingPattern := regexp.MustCompile(`(?is)<h([1-6])\b[^>]*>(.*?)</h[1-6]\s*>`)
	blockPattern := regexp.MustCompile(`(?is)</?(?:address|article|aside|blockquote|br|dd|div|dl|dt|fieldset|figcaption|figure|footer|form|header|hr|li|main|nav|ol|p|pre|section|table|tbody|td|tfoot|th|thead|tr|ul)\b[^>]*>`)

	title := ""
	if match := titlePattern.FindStringSubmatch(raw); len(match) == 2 {
		title = collapseInlineWhitespace(html.UnescapeString(tagPattern.ReplaceAllString(match[1], " ")))
	}
	text := commentPattern.ReplaceAllString(raw, " ")
	text = scriptPattern.ReplaceAllString(text, " ")
	text = stylePattern.ReplaceAllString(text, " ")
	text = headingPattern.ReplaceAllStringFunc(text, func(fragment string) string {
		match := headingPattern.FindStringSubmatch(fragment)
		if len(match) != 3 {
			return "\n"
		}
		heading := collapseInlineWhitespace(html.UnescapeString(tagPattern.ReplaceAllString(match[2], " ")))
		if heading == "" {
			return "\n"
		}
		return "\n\n" + strings.Repeat("#", int(match[1][0]-'0')) + " " + heading + "\n\n"
	})
	text = blockPattern.ReplaceAllString(text, "\n")
	text = tagPattern.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	return normalizeHTMLWhitespace(text), title
}

func normalizeHTMLWhitespace(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	result := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = collapseInlineWhitespace(line)
		if line == "" {
			if !blank && len(result) > 0 {
				result = append(result, "")
			}
			blank = true
			continue
		}
		result = append(result, line)
		blank = false
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

func collapseInlineWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
