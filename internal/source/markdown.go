package source

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

type markdownAdapter struct{}

func (markdownAdapter) Load(_ context.Context, file File) (SourceDocument, error) {
	content, err := readUTF8File(file.Path)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load Markdown source %s: %w", file.Path, err)
	}
	return newDocument(file, content, markdownSections(content), nil), nil
}

func markdownSections(content string) []SourceSection {
	lines := sourceLines(content)
	if len(lines) == 0 {
		return nil
	}
	headingPattern := regexp.MustCompile(`^ {0,3}#{1,6}[\t ]+(.+?)[\t ]*#*[\t ]*$`)
	type headingAt struct {
		line  int
		title string
	}
	var headings []headingAt
	inFence := false
	fenceCharacter := byte(0)
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if len(trimmed) >= 3 && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
			character := trimmed[0]
			if !inFence {
				inFence = true
				fenceCharacter = character
			} else if character == fenceCharacter {
				inFence = false
			}
			continue
		}
		if inFence {
			continue
		}
		match := headingPattern.FindStringSubmatch(line)
		if len(match) == 2 {
			headings = append(headings, headingAt{line: i + 1, title: strings.TrimSpace(match[1])})
		}
	}
	if len(headings) == 0 {
		return []SourceSection{{Content: strings.TrimSpace(strings.Join(lines, "\n")), StartLine: 1, EndLine: len(lines)}}
	}

	sections := make([]SourceSection, 0, len(headings)+1)
	if headings[0].line > 1 {
		contentBefore := strings.TrimSpace(strings.Join(lines[:headings[0].line-1], "\n"))
		if contentBefore != "" {
			sections = append(sections, SourceSection{Content: contentBefore, StartLine: 1, EndLine: headings[0].line - 1})
		}
	}
	for i, heading := range headings {
		endLine := len(lines)
		if i+1 < len(headings) {
			endLine = headings[i+1].line - 1
		}
		contentStart := heading.line + 1
		body := ""
		if contentStart <= endLine {
			body = strings.TrimSpace(strings.Join(lines[contentStart-1:endLine], "\n"))
		}
		sections = append(sections, SourceSection{
			Heading:   heading.title,
			Content:   body,
			StartLine: heading.line,
			EndLine:   endLine,
		})
	}
	return sections
}
