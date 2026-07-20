// Package studio coordinates Noli's end-to-end ingestion workflow.
package studio

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"noli/internal/project"
)

// ImportPaths copies supported source files into a workspace input directory.
// Directories contribute their contents recursively; symbolic links are never
// followed. Returned paths and warnings are deterministic.
func ImportPaths(inputDirectory string, sourcePaths []string) ([]string, []string, error) {
	if len(sourcePaths) == 0 {
		return nil, nil, fmt.Errorf("import sources: at least one source path is required")
	}
	if err := os.MkdirAll(inputDirectory, 0o755); err != nil {
		return nil, nil, fmt.Errorf("import sources: create input directory %s: %w", inputDirectory, err)
	}

	type candidate struct {
		source      string
		destination string
	}
	var candidates []candidate
	var warnings []string
	for _, supplied := range sourcePaths {
		path := filepath.Clean(supplied)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, warnings, fmt.Errorf("import source %s: inspect path: %w", supplied, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			warnings = append(warnings, fmt.Sprintf("skipped symbolic link %s", supplied))
			continue
		}
		if !info.IsDir() {
			if !supportedSourceExtension(path) {
				warnings = append(warnings, fmt.Sprintf("skipped unsupported file %s", supplied))
				continue
			}
			destination, err := project.SafeJoin(inputDirectory, filepath.Base(path))
			if err != nil {
				return nil, warnings, fmt.Errorf("import source %s: resolve destination: %w", supplied, err)
			}
			candidates = append(candidates, candidate{source: path, destination: destination})
			continue
		}

		err = filepath.WalkDir(path, func(filename string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return fmt.Errorf("walk %s: %w", filename, walkErr)
			}
			if entry.Type()&os.ModeSymlink != 0 {
				warnings = append(warnings, fmt.Sprintf("skipped symbolic link %s", filename))
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if !supportedSourceExtension(filename) {
				warnings = append(warnings, fmt.Sprintf("skipped unsupported file %s", filename))
				return nil
			}
			relative, err := filepath.Rel(path, filename)
			if err != nil {
				return fmt.Errorf("relativize %s: %w", filename, err)
			}
			destination, err := project.SafeJoin(inputDirectory, relative)
			if err != nil {
				return fmt.Errorf("resolve destination for %s: %w", filename, err)
			}
			candidates = append(candidates, candidate{source: filename, destination: destination})
			return nil
		})
		if err != nil {
			return nil, warnings, fmt.Errorf("import source directory %s: %w", supplied, err)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].destination == candidates[j].destination {
			return candidates[i].source < candidates[j].source
		}
		return candidates[i].destination < candidates[j].destination
	})
	for i := 1; i < len(candidates); i++ {
		if candidates[i-1].destination == candidates[i].destination {
			return nil, warnings, fmt.Errorf("import sources: %s and %s map to the same destination %s", candidates[i-1].source, candidates[i].source, candidates[i].destination)
		}
	}

	imported := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate.source)
		if err != nil {
			return imported, warnings, fmt.Errorf("import source %s: read: %w", candidate.source, err)
		}
		mode := fs.FileMode(0o644)
		if info, statErr := os.Stat(candidate.source); statErr == nil {
			mode = info.Mode().Perm()
			if mode&0o222 == 0 {
				mode = 0o644
			}
		}
		if err := project.AtomicWriteFile(candidate.destination, data, mode); err != nil {
			return imported, warnings, fmt.Errorf("import source %s to %s: %w", candidate.source, candidate.destination, err)
		}
		relative, err := filepath.Rel(inputDirectory, candidate.destination)
		if err != nil {
			return imported, warnings, fmt.Errorf("import source %s: report destination: %w", candidate.source, err)
		}
		imported = append(imported, filepath.ToSlash(relative))
	}
	sort.Strings(imported)
	sort.Strings(warnings)
	return imported, warnings, nil
}

func supportedSourceExtension(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".md", ".txt", ".json", ".html", ".htm", ".pdf":
		return true
	default:
		return false
	}
}

// IsNotExist reports whether an import failure was caused by a missing path.
// It is kept small so command error handling need not inspect wrapped details.
func IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
