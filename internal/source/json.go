package source

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type jsonAdapter struct{}

func (jsonAdapter) Load(_ context.Context, file File) (SourceDocument, error) {
	raw, err := readUTF8File(file.Path)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load JSON source %s: %w", file.Path, err)
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return SourceDocument{}, fmt.Errorf("load JSON source %s: decode JSON: %w", file.Path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return SourceDocument{}, fmt.Errorf("load JSON source %s: multiple JSON values are not allowed", file.Path)
		}
		return SourceDocument{}, fmt.Errorf("load JSON source %s: decode trailing content: %w", file.Path, err)
	}
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load JSON source %s: normalize JSON: %w", file.Path, err)
	}
	pretty = append(bytes.TrimSpace(pretty), '\n')
	rootType := "scalar"
	switch value.(type) {
	case map[string]any:
		rootType = "object"
	case []any:
		rootType = "array"
	}
	content := string(pretty)
	lines := sourceLines(content)
	section := SourceSection{Heading: "JSON", Content: strings.TrimSpace(content), StartLine: 1, EndLine: len(lines)}
	return newDocument(file, content, []SourceSection{section}, map[string]any{"json_root_type": rootType}), nil
}
