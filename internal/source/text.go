package source

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

type textAdapter struct{}

func (textAdapter) Load(_ context.Context, file File) (SourceDocument, error) {
	content, err := readUTF8File(file.Path)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load text source %s: %w", file.Path, err)
	}
	lines := sourceLines(content)
	sections := []SourceSection(nil)
	if content != "" {
		sections = []SourceSection{{
			Content:   strings.TrimSpace(content),
			StartLine: 1,
			EndLine:   len(lines),
		}}
	}
	return newDocument(file, content, sections, nil), nil
}

func readUTF8File(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filename, err)
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("read %s: content is not valid UTF-8", filename)
	}
	content := string(data)
	content = strings.TrimPrefix(content, "\ufeff")
	return content, nil
}

func sourceLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func newDocument(file File, content string, sections []SourceSection, metadata map[string]any) SourceDocument {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	return SourceDocument{
		ID:        file.ID,
		Name:      file.Name,
		SourceURI: file.SourceURI,
		MediaType: file.MediaType,
		Content:   content,
		Sections:  sections,
		Metadata:  metadata,
	}
}
