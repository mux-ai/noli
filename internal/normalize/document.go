package normalize

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"noli/internal/source"
)

type Document = source.SourceDocument
type Section = source.SourceSection

type Manifest struct {
	Version   int             `json:"version"`
	Documents []ManifestEntry `json:"documents"`
}

type ManifestEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SourceURI string `json:"source_uri"`
	File      string `json:"file"`
}

func NormalizeDocument(document Document) (Document, error) {
	document.ID = strings.TrimSpace(document.ID)
	document.Name = strings.TrimSpace(document.Name)
	document.SourceURI = strings.TrimSpace(document.SourceURI)
	document.MediaType = strings.ToLower(strings.TrimSpace(document.MediaType))
	if err := validateDocumentIdentity(document); err != nil {
		return Document{}, err
	}
	content, err := CleanText(document.Content)
	if err != nil {
		return Document{}, fmt.Errorf("normalize source %s content: %w", document.ID, err)
	}
	document.Content = content
	sections := make([]Section, 0, len(document.Sections))
	for i, section := range document.Sections {
		section.Heading = CollapseWhitespace(section.Heading)
		cleaned, err := CleanText(section.Content)
		if err != nil {
			return Document{}, fmt.Errorf("normalize source %s section %d: %w", document.ID, i, err)
		}
		section.Content = cleaned
		if section.Page < 0 || section.StartLine < 0 || section.EndLine < 0 {
			return Document{}, fmt.Errorf("normalize source %s section %d: page and line numbers must not be negative", document.ID, i)
		}
		if section.StartLine > 0 && section.EndLine > 0 && section.EndLine < section.StartLine {
			return Document{}, fmt.Errorf("normalize source %s section %d: end line precedes start line", document.ID, i)
		}
		if section.Heading == "" && section.Content == "" {
			continue
		}
		sections = append(sections, section)
	}
	if len(sections) == 0 && document.Content != "" {
		sections = []Section{{Content: document.Content, StartLine: 1, EndLine: lineCount(document.Content)}}
	}
	document.Sections = sections
	if document.Metadata == nil {
		document.Metadata = make(map[string]any)
	}
	if _, err := json.Marshal(document.Metadata); err != nil {
		return Document{}, fmt.Errorf("normalize source %s metadata: values must be JSON-compatible: %w", document.ID, err)
	}
	return document, nil
}

func NormalizeDocuments(documents []Document) ([]Document, error) {
	result := make([]Document, 0, len(documents))
	seen := make(map[string]struct{}, len(documents))
	for _, document := range documents {
		normalized, err := NormalizeDocument(document)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[normalized.ID]; exists {
			return nil, fmt.Errorf("normalize documents: duplicate source ID %q", normalized.ID)
		}
		seen[normalized.ID] = struct{}{}
		result = append(result, normalized)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// WriteDocuments replaces staging/normalized only after every normalized
// document and its manifest have been encoded successfully.
func WriteDocuments(directory string, documents []Document) (returnErr error) {
	normalized, err := NormalizeDocuments(documents)
	if err != nil {
		return fmt.Errorf("write normalized documents to %s: %w", directory, err)
	}
	abs, err := filepath.Abs(directory)
	if err != nil {
		return fmt.Errorf("write normalized documents to %s: resolve path: %w", directory, err)
	}
	abs = filepath.Clean(abs)
	parent := filepath.Dir(abs)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("write normalized documents to %s: create parent: %w", directory, err)
	}
	if info, err := os.Lstat(abs); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("write normalized documents to %s: destination must be a non-symlink directory", directory)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("write normalized documents to %s: inspect destination: %w", directory, err)
	}

	temporary, err := os.MkdirTemp(parent, ".noli-normalized-build-*")
	if err != nil {
		return fmt.Errorf("write normalized documents to %s: create temporary directory: %w", directory, err)
	}
	defer func() {
		if removeErr := os.RemoveAll(temporary); removeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("write normalized documents to %s: clean temporary directory: %w", directory, removeErr)
		}
	}()

	manifest := Manifest{Version: 1, Documents: make([]ManifestEntry, 0, len(normalized))}
	for _, document := range normalized {
		filename, err := normalizedFilename(document.ID)
		if err != nil {
			return fmt.Errorf("write normalized documents to %s: %w", directory, err)
		}
		data, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			return fmt.Errorf("write normalized source %s: encode JSON: %w", document.ID, err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(filepath.Join(temporary, filename), data, 0o644); err != nil {
			return fmt.Errorf("write normalized source %s: %w", document.ID, err)
		}
		manifest.Documents = append(manifest.Documents, ManifestEntry{
			ID: document.ID, Name: document.Name, SourceURI: document.SourceURI, File: filename,
		})
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("write normalized manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(temporary, "manifest.json"), append(manifestData, '\n'), 0o644); err != nil {
		return fmt.Errorf("write normalized manifest: %w", err)
	}
	if err := replaceDirectory(abs, temporary); err != nil {
		return fmt.Errorf("write normalized documents to %s: %w", directory, err)
	}
	temporary = ""
	return nil
}

func LoadDocuments(directory string) ([]Document, error) {
	manifestData, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("load normalized manifest from %s: %w", directory, err)
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(manifestData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("load normalized manifest from %s: decode: %w", directory, err)
	}
	if err := requireEOF(decoder); err != nil {
		return nil, fmt.Errorf("load normalized manifest from %s: %w", directory, err)
	}
	if manifest.Version != 1 {
		return nil, fmt.Errorf("load normalized manifest from %s: unsupported version %d", directory, manifest.Version)
	}
	entries := append([]ManifestEntry(nil), manifest.Documents...)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	documents := make([]Document, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if _, exists := seen[entry.ID]; exists {
			return nil, fmt.Errorf("load normalized manifest from %s: duplicate source ID %q", directory, entry.ID)
		}
		seen[entry.ID] = struct{}{}
		expected, err := normalizedFilename(entry.ID)
		if err != nil || entry.File != expected {
			return nil, fmt.Errorf("load normalized manifest from %s: unsafe file for source %q", directory, entry.ID)
		}
		filename := filepath.Join(directory, entry.File)
		info, err := os.Lstat(filename)
		if err != nil {
			return nil, fmt.Errorf("load normalized source %s from %s: %w", entry.ID, filename, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("load normalized source %s from %s: expected a regular non-symlink file", entry.ID, filename)
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("load normalized source %s from %s: %w", entry.ID, filename, err)
		}
		var document Document
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&document); err != nil {
			return nil, fmt.Errorf("load normalized source %s from %s: decode: %w", entry.ID, filename, err)
		}
		if err := requireEOF(decoder); err != nil {
			return nil, fmt.Errorf("load normalized source %s from %s: %w", entry.ID, filename, err)
		}
		if document.ID != entry.ID || document.Name != entry.Name || document.SourceURI != entry.SourceURI {
			return nil, fmt.Errorf("load normalized source %s from %s: manifest fields do not match document", entry.ID, filename)
		}
		normalized, err := NormalizeDocument(document)
		if err != nil {
			return nil, fmt.Errorf("load normalized source %s from %s: %w", entry.ID, filename, err)
		}
		documents = append(documents, normalized)
	}
	return documents, nil
}

func validateDocumentIdentity(document Document) error {
	if document.ID == "" {
		return fmt.Errorf("normalize source: ID must not be empty")
	}
	if strings.ContainsAny(document.ID, "/\\\x00") || document.ID == "." || document.ID == ".." {
		return fmt.Errorf("normalize source %q: ID contains unsafe path characters", document.ID)
	}
	if document.Name == "" {
		return fmt.Errorf("normalize source %s: name must not be empty", document.ID)
	}
	if document.SourceURI == "" {
		return fmt.Errorf("normalize source %s: source URI must not be empty", document.ID)
	}
	if document.MediaType == "" {
		return fmt.Errorf("normalize source %s: media type must not be empty", document.ID)
	}
	if !utf8.ValidString(document.Content) {
		return fmt.Errorf("normalize source %s: content is not valid UTF-8", document.ID)
	}
	return nil
}

func normalizedFilename(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, "/\\\x00") || id == "." || id == ".." {
		return "", fmt.Errorf("source ID %q is unsafe for normalized persistence", id)
	}
	return id + ".json", nil
}

func replaceDirectory(destination, replacement string) error {
	if _, err := os.Lstat(destination); errors.Is(err, os.ErrNotExist) {
		if err := os.Rename(replacement, destination); err != nil {
			return fmt.Errorf("activate normalized directory: %w", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect existing normalized directory: %w", err)
	}
	backup, err := os.MkdirTemp(filepath.Dir(destination), ".noli-normalized-backup-*")
	if err != nil {
		return fmt.Errorf("create normalized backup name: %w", err)
	}
	if err := os.Remove(backup); err != nil {
		return fmt.Errorf("prepare normalized backup path: %w", err)
	}
	if err := os.Rename(destination, backup); err != nil {
		return fmt.Errorf("move current normalized directory to backup: %w", err)
	}
	if err := os.Rename(replacement, destination); err != nil {
		if restoreErr := os.Rename(backup, destination); restoreErr != nil {
			return fmt.Errorf("activate normalized directory: %w (also failed to restore backup: %v)", err, restoreErr)
		}
		return fmt.Errorf("activate normalized directory: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove prior normalized directory backup %s: %w", backup, err)
	}
	return nil
}

func requireEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("multiple JSON values are not allowed")
	}
	return err
}

func lineCount(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(value, "\n") + 1
}
