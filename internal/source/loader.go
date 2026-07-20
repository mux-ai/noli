package source

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultMaximumFileBytes int64 = 64 << 20

type Loader struct {
	PDFCommand       string
	MaximumFileBytes int64
	MaximumPDFOutput int64
}

func NewLoader() Loader {
	return Loader{MaximumFileBytes: DefaultMaximumFileBytes}
}

func SupportedExtensions() []string {
	return []string{".htm", ".html", ".json", ".md", ".pdf", ".txt"}
}

func (l Loader) LoadDirectory(ctx context.Context, inputDirectory string) (LoadResult, error) {
	root, err := canonicalRoot(inputDirectory)
	if err != nil {
		return LoadResult{}, fmt.Errorf("load source directory %s: %w", inputDirectory, err)
	}
	var paths []string
	var warnings []Warning
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk source path %s: %w", path, walkErr)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		var info fs.FileInfo
		if entry.Type()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("resolve source symlink %s: %w", path, err)
			}
			if _, err := containedRelative(root, resolved); err != nil {
				return fmt.Errorf("reject source symlink %s: %w", path, err)
			}
			info, err = os.Stat(path)
			if err != nil {
				return fmt.Errorf("inspect source symlink %s: %w", path, err)
			}
			if info.IsDir() {
				warnings = append(warnings, Warning{Path: sourceURI(root, path), Message: "skipped symlinked directory"})
				return nil
			}
		}
		if info == nil {
			info, err = entry.Info()
			if err != nil {
				return fmt.Errorf("inspect source path %s: %w", path, err)
			}
		}
		if !info.Mode().IsRegular() {
			warnings = append(warnings, Warning{Path: sourceURI(root, path), Message: "skipped non-regular file"})
			return nil
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if !isSupported(extension) {
			warnings = append(warnings, Warning{Path: sourceURI(root, path), Message: fmt.Sprintf("unsupported source extension %q; file skipped", extension)})
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return LoadResult{}, fmt.Errorf("load source directory %s: %w", inputDirectory, err)
	}
	sort.Strings(paths)
	sort.SliceStable(warnings, func(i, j int) bool {
		if warnings[i].Path == warnings[j].Path {
			return warnings[i].Message < warnings[j].Message
		}
		return warnings[i].Path < warnings[j].Path
	})
	documents := make([]SourceDocument, 0, len(paths))
	for _, path := range paths {
		document, err := l.loadFile(ctx, root, path)
		if err != nil {
			return LoadResult{}, fmt.Errorf("load source directory %s: %w", inputDirectory, err)
		}
		documents = append(documents, document)
	}
	sort.SliceStable(documents, func(i, j int) bool { return documents[i].ID < documents[j].ID })
	return LoadResult{Documents: documents, Warnings: warnings}, nil
}

// LoadDirectory is the convenient default-loader entry point.
func LoadDirectory(ctx context.Context, inputDirectory string) ([]SourceDocument, []Warning, error) {
	result, err := NewLoader().LoadDirectory(ctx, inputDirectory)
	return result.Documents, result.Warnings, err
}

func (l Loader) LoadFile(ctx context.Context, inputDirectory, filename string) (SourceDocument, error) {
	root, err := canonicalRoot(inputDirectory)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load source file %s: resolve input root: %w", filename, err)
	}
	return l.loadFile(ctx, root, filename)
}

func (l Loader) loadFile(ctx context.Context, root, filename string) (SourceDocument, error) {
	pathToResolve := filename
	if !filepath.IsAbs(pathToResolve) {
		pathToResolve = filepath.Join(root, pathToResolve)
	}
	absolute, err := filepath.Abs(pathToResolve)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load source file %s: resolve absolute path: %w", filename, err)
	}
	absolute = filepath.Clean(absolute)
	relative, err := containedRelative(root, absolute)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load source file %s: %w", filename, err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load source file %s: resolve symlinks: %w", filename, err)
	}
	if _, err := containedRelative(root, resolved); err != nil {
		return SourceDocument{}, fmt.Errorf("load source file %s: resolved path escapes input directory: %w", filename, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load source file %s: inspect file: %w", filename, err)
	}
	if !info.Mode().IsRegular() {
		return SourceDocument{}, fmt.Errorf("load source file %s: not a regular file", filename)
	}
	maximum := l.MaximumFileBytes
	if maximum <= 0 {
		maximum = DefaultMaximumFileBytes
	}
	if info.Size() > maximum {
		return SourceDocument{}, fmt.Errorf("load source file %s: file size %d exceeds limit %d", filename, info.Size(), maximum)
	}
	extension := strings.ToLower(filepath.Ext(relative))
	adapter, mediaType, err := l.adapter(extension)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load source file %s: %w", filename, err)
	}
	uri := filepath.ToSlash(relative)
	file := File{
		ID:        StableSourceID(uri),
		Name:      filepath.Base(relative),
		Path:      resolved,
		SourceURI: uri,
		MediaType: mediaType,
	}
	document, err := adapter.Load(ctx, file)
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load source file %s with %s adapter: %w", filename, extension, err)
	}
	return document, nil
}

func (l Loader) adapter(extension string) (Adapter, string, error) {
	switch extension {
	case ".md":
		return markdownAdapter{}, "text/markdown", nil
	case ".txt":
		return textAdapter{}, "text/plain", nil
	case ".json":
		return jsonAdapter{}, "application/json", nil
	case ".html", ".htm":
		return htmlAdapter{}, "text/html", nil
	case ".pdf":
		return PDFAdapter{Command: l.PDFCommand, MaxOutputBytes: l.MaximumPDFOutput}, "application/pdf", nil
	default:
		return nil, "", fmt.Errorf("unsupported source extension %q", extension)
	}
}

func StableSourceID(sourceURI string) string {
	normalized := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(sourceURI)), "./")
	digest := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("source-%x", digest[:10])
}

func canonicalRoot(directory string) (string, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve directory symlinks: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory")
	}
	return filepath.Clean(resolved), nil
}

func containedRelative(root, target string) (string, error) {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return "", fmt.Errorf("check path containment: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path %s escapes source root %s", target, root)
	}
	return relative, nil
}

func sourceURI(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func isSupported(extension string) bool {
	for _, supported := range SupportedExtensions() {
		if extension == supported {
			return true
		}
	}
	return false
}
