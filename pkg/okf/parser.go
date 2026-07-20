package okf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Parse problem codes. They reuse the frozen protocol error-code vocabulary
// (docs/PROTOCOL.md sections 3 and 8).
const (
	CodeParseError         = "PARSE_ERROR"
	CodeInvalidFrontmatter = "INVALID_FRONTMATTER"
	CodeDuplicateID        = "DUPLICATE_ID"
)

// Default parser bounds, applied when the corresponding option is zero.
const (
	DefaultMaxFileBytes  int64 = 2 << 20  // 2 MiB per document
	DefaultMaxTotalBytes int64 = 64 << 20 // 64 MiB per bundle
	DefaultMaxDocuments        = 10000
)

// sensitiveComponents are path components that are never loaded, in addition
// to any component starting with a dot (docs/PROTOCOL.md section 9).
var sensitiveComponents = []string{
	"node_modules", "vendor", "build", "secrets", "credentials",
}

// ParseOptions bounds and filters bundle loading. Zero values select the
// package defaults; negative bounds are invalid.
type ParseOptions struct {
	// MaxFileBytes caps a single document file size.
	MaxFileBytes int64
	// MaxTotalBytes caps the aggregate size of all loaded documents.
	MaxTotalBytes int64
	// MaxDocuments caps the number of Markdown files in the bundle.
	MaxDocuments int
	// Exclude lists additional exclusions. An entry without a slash matches
	// any path component case-insensitively; an entry with slashes matches a
	// root-relative slash path prefix case-insensitively.
	Exclude []string
}

func (o ParseOptions) withDefaults() (ParseOptions, error) {
	if o.MaxFileBytes < 0 || o.MaxTotalBytes < 0 || o.MaxDocuments < 0 {
		return o, errors.New("parse bounds must not be negative")
	}
	if o.MaxFileBytes == 0 {
		o.MaxFileBytes = DefaultMaxFileBytes
	}
	if o.MaxTotalBytes == 0 {
		o.MaxTotalBytes = DefaultMaxTotalBytes
	}
	if o.MaxDocuments == 0 {
		o.MaxDocuments = DefaultMaxDocuments
	}
	return o, nil
}

// ParseProblem is one independently reportable parse failure.
type ParseProblem struct {
	// Document is the root-relative slash path (or ID when known); empty for
	// bundle-level problems.
	Document string `json:"document"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// ParseErrors aggregates every independently parseable document failure.
type ParseErrors struct {
	Problems []ParseProblem
}

func (e *ParseErrors) Error() string {
	if e == nil || len(e.Problems) == 0 {
		return "parse failed"
	}
	parts := make([]string, len(e.Problems))
	for i, problem := range e.Problems {
		if problem.Document != "" {
			parts[i] = problem.Document + ": " + problem.Message
		} else {
			parts[i] = problem.Message
		}
	}
	return strings.Join(parts, "; ")
}

// ParsedBundle contains documents indexed by their root-relative ID.
type ParsedBundle struct {
	// Root is the absolute, symlink-resolved knowledge root.
	Root      string
	Documents map[string]Document
	// Order lists document IDs sorted ascending.
	Order []string
	// BundleID is "sha256:<hex>" over sorted IDs and normalized document
	// bytes of the successfully parsed documents.
	BundleID string
}

func (b *ParsedBundle) Document(id string) (Document, bool) {
	document, ok := b.Documents[id]
	return document, ok
}

// ParseBundle recursively parses every Markdown file below root, applying
// exclusions and size bounds. It returns the bundle together with a
// *ParseErrors when any document fails; hard errors (bad root, exceeded
// bundle bounds) return a nil bundle.
func ParseBundle(root string, options ParseOptions) (*ParsedBundle, error) {
	options, err := options.withDefaults()
	if err != nil {
		return nil, err
	}
	absRoot, err := resolveRoot(root)
	if err != nil {
		return nil, err
	}

	parseErrors := &ParseErrors{}
	var paths []string
	var totalBytes int64
	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", path, walkErr)
		}
		if path == absRoot {
			return nil
		}
		rel, relErr := filepath.Rel(absRoot, path)
		if relErr != nil {
			return fmt.Errorf("relativize %q: %w", path, relErr)
		}
		if isExcluded(rel, options.Exclude) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil // never follow symlinks inside the bundle
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return fmt.Errorf("stat %q: %w", path, infoErr)
		}
		if info.Size() > options.MaxFileBytes {
			parseErrors.Problems = append(parseErrors.Problems, ParseProblem{
				Document: filepath.ToSlash(rel),
				Code:     CodeParseError,
				Message:  fmt.Sprintf("document exceeds the %d byte file limit", options.MaxFileBytes),
			})
			return nil
		}
		totalBytes += info.Size()
		if totalBytes > options.MaxTotalBytes {
			return fmt.Errorf("knowledge bundle exceeds the %d byte total size limit", options.MaxTotalBytes)
		}
		paths = append(paths, path)
		if len(paths) > options.MaxDocuments {
			return fmt.Errorf("knowledge bundle exceeds the %d document limit", options.MaxDocuments)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover Markdown documents in %q: %w", root, err)
	}
	sort.Strings(paths)

	bundle := &ParsedBundle{Root: absRoot, Documents: make(map[string]Document)}
	normalized := make(map[string]string)
	for _, path := range paths {
		rel, relErr := filepath.Rel(absRoot, path)
		if relErr != nil {
			return nil, fmt.Errorf("relativize %q: %w", path, relErr)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("read Markdown document %q: %w", rel, readErr)
		}
		document, text, code, parseErr := parseDocumentBytes(absRoot, rel, data)
		if parseErr != nil {
			parseErrors.Problems = append(parseErrors.Problems, ParseProblem{
				Document: filepath.ToSlash(rel),
				Code:     code,
				Message:  parseErr.Error(),
			})
			continue
		}
		if previous, exists := bundle.Documents[document.ID]; exists {
			parseErrors.Problems = append(parseErrors.Problems, ParseProblem{
				Document: document.ID,
				Code:     CodeDuplicateID,
				Message:  fmt.Sprintf("duplicate document ID %q from %q and %q", document.ID, previous.Path, document.Path),
			})
			continue
		}
		bundle.Documents[document.ID] = document
		bundle.Order = append(bundle.Order, document.ID)
		normalized[document.ID] = text
	}
	sort.Strings(bundle.Order)
	bundle.BundleID = bundleChecksum(bundle.Order, normalized)
	if len(parseErrors.Problems) > 0 {
		return bundle, parseErrors
	}
	return bundle, nil
}

// ParseDocument parses one Markdown document. The path must stay inside root
// after symlink resolution.
func ParseDocument(root, path string) (Document, error) {
	absRoot, err := resolveRoot(root)
	if err != nil {
		return Document{}, err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Document{}, fmt.Errorf("resolve document path %q: %w", path, err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return Document{}, fmt.Errorf("resolve document path %q: %w", path, err)
	}
	rel, err := filepath.Rel(absRoot, resolvedPath)
	if err != nil {
		return Document{}, fmt.Errorf("relativize document path %q: %w", path, err)
	}
	if escapesRoot(rel) {
		return Document{}, fmt.Errorf("document path %q escapes knowledge root %q", path, root)
	}
	rel = filepath.Clean(rel)
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return Document{}, fmt.Errorf("read Markdown document %q: %w", rel, err)
	}
	if int64(len(data)) > DefaultMaxFileBytes {
		return Document{}, fmt.Errorf("document %q exceeds the %d byte file limit", rel, DefaultMaxFileBytes)
	}
	document, _, _, parseErr := parseDocumentBytes(absRoot, rel, data)
	if parseErr != nil {
		return Document{}, fmt.Errorf("parse %q: %w", rel, parseErr)
	}
	return document, nil
}

// parseDocumentBytes parses document content and returns the document, the
// normalized text used for checksums, and a stable problem code on failure.
func parseDocumentBytes(absRoot, rel string, data []byte) (Document, string, string, error) {
	if !utf8.Valid(data) {
		return Document{}, "", CodeParseError, errors.New("content is not valid UTF-8")
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	base := filepath.Base(rel)
	isIndex := strings.EqualFold(base, "index.md")
	// OKF v0.1 section 7: log.md may appear at any level of the hierarchy.
	isLog := strings.EqualFold(base, "log.md")

	var metadata Metadata
	body := text
	// OKF v0.1 conformance rule 1 requires frontmatter only on non-reserved
	// documents. Section 6 states index files carry no frontmatter; log
	// files may omit it too. Reserved files that do carry frontmatter are
	// still parsed so mixed-producer bundles keep working.
	if !isIndex && !isLog {
		parsed, remainder, code, err := parseFrontmatter(text)
		if err != nil {
			return Document{}, "", code, err
		}
		metadata, body = parsed, remainder
	} else if strings.HasPrefix(text, "---\n") {
		parsed, remainder, code, err := parseFrontmatter(text)
		if err != nil {
			return Document{}, "", code, err
		}
		metadata, body = parsed, remainder
	}

	links, err := ExtractLinks(absRoot, filepath.ToSlash(rel), body)
	if err != nil {
		return Document{}, "", CodeParseError, fmt.Errorf("extract links: %w", err)
	}

	id := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
	return Document{
		ID:       id,
		Path:     filepath.ToSlash(rel),
		Metadata: metadata,
		Body:     body,
		Links:    links,
		IsIndex:  isIndex,
		IsLog:    isLog,
	}, text, "", nil
}

// parseFrontmatter splits normalized text into metadata and body. The second
// return is the body; the third is a stable problem code on failure.
func parseFrontmatter(text string) (Metadata, string, string, error) {
	if !strings.HasPrefix(text, "---\n") {
		return Metadata{}, "", CodeParseError, errors.New("missing opening YAML frontmatter delimiter")
	}
	rest := text[len("---\n"):]
	frontmatter, after, found := strings.Cut(rest, "\n---")
	if !found {
		return Metadata{}, "", CodeParseError, errors.New("missing closing YAML frontmatter delimiter")
	}
	if after != "" && after[0] != '\n' {
		return Metadata{}, "", CodeParseError, errors.New("closing YAML frontmatter delimiter must occupy its own line")
	}
	if strings.TrimSpace(frontmatter) == "" {
		return Metadata{}, "", CodeInvalidFrontmatter, errors.New("empty YAML frontmatter")
	}
	var metadata Metadata
	decoder := yaml.NewDecoder(strings.NewReader(frontmatter))
	if err := decoder.Decode(&metadata); err != nil {
		return Metadata{}, "", CodeInvalidFrontmatter, fmt.Errorf("decode YAML: %w", err)
	}
	var trailing any
	switch err := decoder.Decode(&trailing); {
	case err == nil:
		return Metadata{}, "", CodeInvalidFrontmatter, errors.New("frontmatter contains multiple YAML documents")
	case !errors.Is(err, io.EOF):
		return Metadata{}, "", CodeInvalidFrontmatter, fmt.Errorf("trailing frontmatter content: %w", err)
	}
	body := ""
	if after != "" {
		body = strings.TrimPrefix(after, "\n")
	}
	return metadata, body, "", nil
}

// resolveRoot validates the knowledge root and resolves symlinks, so all
// later containment checks compare against the real directory.
func resolveRoot(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve knowledge root %q: %w", root, err)
	}
	resolved, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("resolve knowledge root %q: %w", root, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat knowledge root %q: %w", root, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("knowledge root %q is not a directory", root)
	}
	return resolved, nil
}

func escapesRoot(rel string) bool {
	clean := filepath.Clean(rel)
	return clean == ".." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

// IsSensitivePath reports whether a root-relative slash path hits the
// built-in sensitive-path rules (hidden components or sensitive names).
func IsSensitivePath(rel string) bool {
	return isExcluded(rel, nil)
}

// IsExcludedPath reports whether the root-relative path is excluded by the
// built-in rules or the supplied exclusions (ParseOptions.Exclude
// semantics).
func IsExcludedPath(rel string, exclude []string) bool {
	return isExcluded(rel, exclude)
}

// isExcluded applies built-in sensitive-path rules plus configured
// exclusions to a root-relative path.
func isExcluded(rel string, exclude []string) bool {
	slashPath := filepath.ToSlash(rel)
	components := strings.Split(slashPath, "/")
	for _, component := range components {
		if strings.HasPrefix(component, ".") {
			return true // hidden entries: .git, .env, .noli previews, dotfiles
		}
		for _, sensitive := range sensitiveComponents {
			if strings.EqualFold(component, sensitive) {
				return true
			}
		}
	}
	lowerPath := strings.ToLower(slashPath)
	for _, entry := range exclude {
		normalized := strings.ToLower(strings.Trim(strings.TrimSpace(filepath.ToSlash(entry)), "/"))
		if normalized == "" {
			continue
		}
		if strings.Contains(normalized, "/") {
			if lowerPath == normalized || strings.HasPrefix(lowerPath, normalized+"/") {
				return true
			}
			continue
		}
		for _, component := range components {
			if strings.EqualFold(component, normalized) {
				return true
			}
		}
	}
	return false
}

func bundleChecksum(order []string, normalized map[string]string) string {
	hash := sha256.New()
	for _, id := range order {
		hash.Write([]byte(id))
		hash.Write([]byte{0})
		hash.Write([]byte(normalized[id]))
		hash.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
