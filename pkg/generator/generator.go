package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"noli/pkg/internal/targetlock"
	"noli/pkg/okf"
)

// DefaultPreviewDir is the dry-run output directory relative to noli.yaml.
const DefaultPreviewDir = ".noli/preview"

// ErrWriteBusy reports that another process is generating or applying output
// for the same project. Callers may retry after the other writer completes.
var ErrWriteBusy = targetlock.ErrBusy

// BundleValidationError reports that the generated bundle failed validation;
// active knowledge is left byte-for-byte unchanged.
type BundleValidationError struct {
	Errors []okf.Problem
}

func (e *BundleValidationError) Error() string {
	return fmt.Sprintf("generated bundle failed validation with %d errors; active knowledge was left unchanged",
		len(e.Errors))
}

// GenerateOptions selects dry-run (default) or apply.
type GenerateOptions struct {
	// Apply replaces the active knowledge root after validation. When false,
	// output goes to the preview directory only.
	Apply bool
	// PreviewDir overrides the dry-run output directory (relative to
	// noli.yaml). Empty means DefaultPreviewDir.
	PreviewDir string
}

// GenerateResult reports the diff between the generated bundle and the
// active knowledge, by document ID with sorted, non-nil slices.
type GenerateResult struct {
	Mode        string
	PreviewRoot string
	Added       []string
	Changed     []string
	Removed     []string
	Unchanged   []string
}

// Generate renders the configured structured concepts into a complete
// knowledge bundle. Dry-run writes only the preview directory; apply builds
// a temporary sibling, validates it, and atomically replaces the active
// root, rolling back on any failure.
func Generate(config *Config, options GenerateOptions) (GenerateResult, error) {
	activeRoot, err := config.KnowledgeRoot()
	if err != nil {
		return GenerateResult{}, err
	}
	lockPath := filepath.Join(config.Dir(), ".noli", "write.lock")
	lock, err := targetlock.Acquire(lockPath)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("generate project output: %w", err)
	}
	defer func() { _ = lock.Release() }()

	// PLANS.md section 5.5 fallback: with no structured inputs, the active
	// bundle is parsed and deterministically re-rendered (source
	// normalization: BOM stripped, line endings normalized).
	documents, err := renderedDocuments(config, activeRoot)
	if err != nil {
		return GenerateResult{}, err
	}
	active, err := hashActiveBundle(activeRoot, config.Security.Exclude)
	if err != nil {
		return GenerateResult{}, err
	}
	result := diffBundles(active, documents)

	if !options.Apply {
		previewRelative := options.PreviewDir
		if previewRelative == "" {
			previewRelative = DefaultPreviewDir
		}
		previewRoot, err := resolvePreviewRoot(config, previewRelative, activeRoot)
		if err != nil {
			return GenerateResult{}, err
		}
		if err := os.RemoveAll(previewRoot); err != nil {
			return GenerateResult{}, fmt.Errorf("clear preview directory %q: %w", previewRelative, err)
		}
		if err := writeBundle(previewRoot, documents); err != nil {
			return GenerateResult{}, err
		}
		result.Mode = "dry-run"
		result.PreviewRoot = previewRelative
		return result, nil
	}

	parent := filepath.Dir(activeRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return GenerateResult{}, fmt.Errorf("create knowledge parent: %w", err)
	}
	temporary, err := os.MkdirTemp(parent, "."+filepath.Base(activeRoot)+".applying-*")
	if err != nil {
		return GenerateResult{}, fmt.Errorf("create temporary directory: %w", err)
	}
	if err := os.Chmod(temporary, 0o755); err != nil {
		_ = os.RemoveAll(temporary)
		return GenerateResult{}, fmt.Errorf("set temporary directory permissions: %w", err)
	}
	defer func() { _ = os.RemoveAll(temporary) }()
	backup := temporary + ".previous"
	if err := writeBundle(temporary, documents); err != nil {
		return GenerateResult{}, err
	}
	report := okf.Validate(temporary, config.ValidationOptions())
	if !report.Valid {
		_ = os.RemoveAll(temporary)
		return GenerateResult{}, &BundleValidationError{Errors: report.Errors}
	}
	activeExists := false
	if _, err := os.Stat(activeRoot); err == nil {
		activeExists = true
		if err := os.Rename(activeRoot, backup); err != nil {
			return GenerateResult{}, fmt.Errorf("stage active knowledge for replacement: %w", err)
		}
	}
	if err := os.Rename(temporary, activeRoot); err != nil {
		if activeExists {
			if rollbackErr := os.Rename(backup, activeRoot); rollbackErr != nil {
				return GenerateResult{}, fmt.Errorf(
					"activate generated knowledge: %v; rollback failed and backup remains at %q: %w",
					err, backup, rollbackErr)
			}
		}
		return GenerateResult{}, fmt.Errorf("activate generated knowledge: %w", err)
	}
	_ = os.RemoveAll(backup)
	result.Mode = "apply"
	result.PreviewRoot = ""
	return result, nil
}

// passthroughBundle parses the active bundle and re-renders it as
// normalized source bytes. It fails when no parseable active bundle exists.
func passthroughBundle(config *Config, activeRoot string) (map[string][]byte, error) {
	bundle, err := okf.ParseBundle(activeRoot, okf.ParseOptions{Exclude: config.Security.Exclude})
	if err != nil {
		return nil, fmt.Errorf("no structured concept inputs are configured and the active bundle cannot be re-rendered: %w", err)
	}
	if len(bundle.Order) == 0 {
		return nil, fmt.Errorf("no structured concept inputs are configured and the active bundle is empty")
	}
	documents := make(map[string][]byte, len(bundle.Order))
	for _, id := range bundle.Order {
		data, readErr := os.ReadFile(filepath.Join(activeRoot, filepath.FromSlash(bundle.Documents[id].Path)))
		if readErr != nil {
			return nil, fmt.Errorf("re-render %q: %w", id, readErr)
		}
		documents[id] = normalizeSource(data)
	}
	return documents, nil
}

// preserveLogs carries hand-authored log.md files from the active bundle
// into the generated document set. Generation never writes logs (OKF v0.1
// section 7 makes them optional and dated), so without this an apply would
// delete a curated history.
func preserveLogs(activeRoot string, exclude []string, documents map[string][]byte) error {
	info, err := os.Stat(activeRoot)
	if err != nil || !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(activeRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", path, walkErr)
		}
		rel, relErr := filepath.Rel(activeRoot, path)
		if relErr != nil {
			return relErr
		}
		if path != activeRoot && okf.IsExcludedPath(rel, exclude) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 ||
			!strings.EqualFold(entry.Name(), "log.md") {
			return nil
		}
		id := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
		if _, generated := documents[id]; generated {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("preserve log %q: %w", id, readErr)
		}
		documents[id] = normalizeSource(data)
		return nil
	})
}

// normalizeSource applies the parser's source normalization: UTF-8 BOM
// stripped and CR/CRLF line endings converted to LF.
func normalizeSource(data []byte) []byte {
	text := strings.TrimPrefix(string(data), "\xef\xbb\xbf")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return []byte(text)
}

// renderBundle renders every concept plus directory indexes, the root
// index, and the static log.
func renderBundle(config *Config, concepts []resolvedConcept) (map[string][]byte, error) {
	titles := make(map[string]string, len(concepts))
	for _, concept := range concepts {
		titles[concept.ID] = concept.Input.Title
	}
	documents := make(map[string][]byte)
	byDirectory := make(map[string][]resolvedConcept)
	directoryTypes := make(map[string]string)
	for _, concept := range concepts {
		data, err := renderConcept(concept, titles)
		if err != nil {
			return nil, err
		}
		documents[concept.ID] = data
		byDirectory[concept.Directory] = append(byDirectory[concept.Directory], concept)
		if _, exists := directoryTypes[concept.Directory]; !exists {
			directoryTypes[concept.Directory] = concept.Type
		}
	}
	counts := make(map[string]int, len(byDirectory))
	for directory, entries := range byDirectory {
		index, err := renderDirectoryIndex(directory, directoryTypes[directory], entries)
		if err != nil {
			return nil, err
		}
		documents[directory+"/index"] = index
		counts[directory] = len(entries)
	}
	rootIndex, err := renderRootIndex(config.Project.Name, directoryTypes, counts)
	if err != nil {
		return nil, err
	}
	documents["index"] = rootIndex
	// No log.md is generated: OKF v0.1 section 7 makes it optional and
	// requires ISO 8601 date headings, which would need an invented clock
	// and would break byte-for-byte reproducibility. Hand-authored logs in
	// the active bundle are preserved by the passthrough path.
	return documents, nil
}

// hashActiveBundle maps document IDs to content checksums for the current
// active knowledge. A missing root is an empty bundle.
func hashActiveBundle(root string, exclude []string) (map[string]string, error) {
	hashes := make(map[string]string)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return hashes, nil
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", path, walkErr)
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if okf.IsExcludedPath(rel, exclude) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() ||
			!strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %q: %w", rel, readErr)
		}
		id := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
		hashes[id] = contentHash(data)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("hash active knowledge: %w", err)
	}
	return hashes, nil
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func diffBundles(active map[string]string, generated map[string][]byte) GenerateResult {
	result := GenerateResult{
		Added:     []string{},
		Changed:   []string{},
		Removed:   []string{},
		Unchanged: []string{},
	}
	for id, data := range generated {
		previous, exists := active[id]
		switch {
		case !exists:
			result.Added = append(result.Added, id)
		case previous != contentHash(data):
			result.Changed = append(result.Changed, id)
		default:
			result.Unchanged = append(result.Unchanged, id)
		}
	}
	for id := range active {
		if _, exists := generated[id]; !exists {
			result.Removed = append(result.Removed, id)
		}
	}
	sort.Strings(result.Added)
	sort.Strings(result.Changed)
	sort.Strings(result.Removed)
	sort.Strings(result.Unchanged)
	return result
}

// resolvePreviewRoot validates the preview directory: it must stay inside
// the project directory and must never overlap the active knowledge root.
func resolvePreviewRoot(config *Config, relative, activeRoot string) (string, error) {
	if err := safeRelativePath("preview directory", relative); err != nil {
		return "", err
	}
	previewRoot := filepath.Join(config.dir, filepath.FromSlash(relative))
	separator := string(filepath.Separator)
	if previewRoot == activeRoot ||
		strings.HasPrefix(previewRoot+separator, activeRoot+separator) ||
		strings.HasPrefix(activeRoot+separator, previewRoot+separator) {
		return "", fmt.Errorf("preview directory %q overlaps the active knowledge root: %w",
			relative, ErrUnsafePath)
	}
	return previewRoot, nil
}

// writeBundle writes every rendered document below root.
func writeBundle(root string, documents map[string][]byte) error {
	ids := make([]string, 0, len(documents))
	for id := range documents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := validateConceptID(id); err != nil {
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(id+".md"))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create directory for %q: %w", id, err)
		}
		if err := os.WriteFile(path, documents[id], 0o644); err != nil {
			return fmt.Errorf("write document %q: %w", id, err)
		}
	}
	return nil
}
