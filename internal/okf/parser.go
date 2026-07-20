package okf

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// ParsedBundle contains documents indexed by their root-relative ID.
type ParsedBundle struct {
	Root      string
	Documents map[string]Document
	Order     []string
}

func (b *ParsedBundle) Document(id string) (Document, bool) {
	document, ok := b.Documents[id]
	return document, ok
}

// ParseErrors reports all independently parseable document failures.
type ParseErrors struct {
	Errors []error
}

func (e *ParseErrors) Error() string {
	if e == nil || len(e.Errors) == 0 {
		return ""
	}
	parts := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		parts[i] = err.Error()
	}
	return strings.Join(parts, "; ")
}

func (e *ParseErrors) Unwrap() []error {
	if e == nil {
		return nil
	}
	return e.Errors
}

// ParseBundle recursively parses every Markdown file below root. Results are
// returned in deterministic ID order.
func ParseBundle(root string) (*ParsedBundle, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve knowledge root %q: %w", root, err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("stat knowledge root %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("knowledge root %q is not a directory", root)
	}

	var paths []string
	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", path, walkErr)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover Markdown documents in %q: %w", root, err)
	}
	sort.Strings(paths)

	bundle := &ParsedBundle{Root: absRoot, Documents: make(map[string]Document)}
	parseErrors := &ParseErrors{}
	for _, path := range paths {
		document, parseErr := ParseDocument(absRoot, path)
		if parseErr != nil {
			parseErrors.Errors = append(parseErrors.Errors, parseErr)
			continue
		}
		if previous, exists := bundle.Documents[document.ID]; exists {
			parseErrors.Errors = append(parseErrors.Errors, fmt.Errorf("duplicate document ID %q from %q and %q", document.ID, previous.Path, document.Path))
			continue
		}
		bundle.Documents[document.ID] = document
		bundle.Order = append(bundle.Order, document.ID)
	}
	if len(parseErrors.Errors) > 0 {
		return bundle, parseErrors
	}
	return bundle, nil
}

// ParseDocument parses one Markdown document and resolves its local links
// against the supplied bundle root.
func ParseDocument(root, path string) (Document, error) {
	absRoot, absPath, rel, err := safeExistingPath(root, path)
	if err != nil {
		return Document{}, err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return Document{}, fmt.Errorf("read Markdown document %q: %w", rel, err)
	}
	if !utf8.Valid(data) {
		return Document{}, fmt.Errorf("parse Markdown document %q: content is not valid UTF-8", rel)
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	metadata, body, err := parseFrontmatter(data)
	if err != nil {
		return Document{}, fmt.Errorf("parse frontmatter in %q: %w", rel, err)
	}
	links, err := ExtractLocalLinks(absRoot, rel, body)
	if err != nil {
		return Document{}, fmt.Errorf("extract links from %q: %w", rel, err)
	}

	base := filepath.Base(rel)
	id := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
	return Document{
		ID:       id,
		Path:     filepath.ToSlash(rel),
		Metadata: metadata,
		Body:     body,
		Links:    links,
		IsIndex:  strings.EqualFold(base, "index.md"),
		IsLog:    strings.EqualFold(base, "log.md") && filepath.Dir(rel) == ".",
	}, nil
}

func parseFrontmatter(data []byte) (Metadata, string, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return Metadata{}, "", errors.New("missing opening YAML frontmatter delimiter")
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return Metadata{}, "", errors.New("missing closing YAML frontmatter delimiter")
	}
	after := rest[end+len("\n---"):]
	if after != "" && after[0] != '\n' {
		return Metadata{}, "", errors.New("closing YAML frontmatter delimiter must occupy its own line")
	}
	frontmatter := rest[:end]
	if strings.TrimSpace(frontmatter) == "" {
		return Metadata{}, "", errors.New("empty YAML frontmatter")
	}
	var metadata Metadata
	decoder := yaml.NewDecoder(strings.NewReader(frontmatter))
	if err := decoder.Decode(&metadata); err != nil {
		return Metadata{}, "", fmt.Errorf("decode YAML: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return Metadata{}, "", errors.New("frontmatter contains multiple YAML documents")
	}
	body := ""
	if after != "" {
		body = strings.TrimPrefix(after, "\n")
	}
	return metadata, body, nil
}

func safeExistingPath(root, path string) (string, string, string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve knowledge root %q: %w", root, err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve document path %q: %w", path, err)
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", "", "", fmt.Errorf("relativize document path %q: %w", path, err)
	}
	if escapesRoot(rel) {
		return "", "", "", fmt.Errorf("document path %q escapes knowledge root %q", path, root)
	}
	return absRoot, absPath, filepath.Clean(rel), nil
}

func escapesRoot(rel string) bool {
	clean := filepath.Clean(rel)
	return clean == ".." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

// ExtractLocalLinks finds local Markdown links and returns normalized target
// IDs. HTTP(S), other URI schemes, images, and non-Markdown assets are ignored.
func ExtractLocalLinks(root, documentPath, body string) ([]string, error) {
	markdownLinkPattern, err := regexp.Compile(`\[[^\]\n]*\]\(([^)\n]+)\)`)
	if err != nil {
		return nil, fmt.Errorf("compile Markdown link parser: %w", err)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve knowledge root %q: %w", root, err)
	}
	currentRel := filepath.Clean(filepath.FromSlash(documentPath))
	if filepath.IsAbs(currentRel) || escapesRoot(currentRel) {
		return nil, fmt.Errorf("document path %q escapes knowledge root", documentPath)
	}

	seen := make(map[string]struct{})
	var targets []string
	for _, match := range markdownLinkPattern.FindAllStringSubmatchIndex(body, -1) {
		if match[0] > 0 && body[match[0]-1] == '!' {
			continue
		}
		raw := strings.TrimSpace(body[match[2]:match[3]])
		destination := markdownDestination(raw)
		if destination == "" || strings.ContainsRune(destination, '\x00') {
			continue
		}
		parsed, parseErr := url.Parse(destination)
		if parseErr != nil {
			return nil, fmt.Errorf("parse link destination %q: %w", destination, parseErr)
		}
		if parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(destination, "//") {
			continue
		}
		decodedPath, decodeErr := url.PathUnescape(parsed.EscapedPath())
		if decodeErr != nil {
			return nil, fmt.Errorf("decode link destination %q: %w", destination, decodeErr)
		}
		if decodedPath == "" {
			continue
		}
		if strings.Contains(decodedPath, "\\") {
			return nil, fmt.Errorf("link destination %q contains a backslash", destination)
		}

		var target string
		if strings.HasPrefix(decodedPath, "/") {
			target = filepath.Join(absRoot, filepath.FromSlash(strings.TrimPrefix(decodedPath, "/")))
		} else {
			target = filepath.Join(absRoot, filepath.Dir(currentRel), filepath.FromSlash(decodedPath))
		}
		if strings.HasSuffix(decodedPath, "/") {
			target = filepath.Join(target, "index.md")
		}
		target = filepath.Clean(target)
		rel, relErr := filepath.Rel(absRoot, target)
		if relErr != nil {
			return nil, fmt.Errorf("resolve link destination %q: %w", destination, relErr)
		}
		if escapesRoot(rel) {
			return nil, fmt.Errorf("link destination %q escapes knowledge root", destination)
		}
		ext := filepath.Ext(rel)
		if ext != "" && !strings.EqualFold(ext, ".md") {
			continue
		}
		id := strings.TrimSuffix(filepath.ToSlash(rel), ext)
		if id == "." || id == "" {
			continue
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			targets = append(targets, id)
		}
	}
	sort.Strings(targets)
	return targets, nil
}

func markdownDestination(raw string) string {
	if strings.HasPrefix(raw, "<") {
		if end := strings.Index(raw, ">"); end > 0 {
			return raw[1:end]
		}
	}
	for i, r := range raw {
		if r == ' ' || r == '\t' {
			return raw[:i]
		}
	}
	return raw
}
