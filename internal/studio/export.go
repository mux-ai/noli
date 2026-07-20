package studio

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ExportBundle writes a deterministic ZIP containing the regular files in a
// knowledge directory. Symbolic links and paths outside the directory are not
// included.
func ExportBundle(knowledgeDirectory, outputFilename string) (returnErr error) {
	root, err := filepath.Abs(knowledgeDirectory)
	if err != nil {
		return fmt.Errorf("export bundle: resolve knowledge directory %s: %w", knowledgeDirectory, err)
	}
	output, err := filepath.Abs(outputFilename)
	if err != nil {
		return fmt.Errorf("export bundle: resolve output %s: %w", outputFilename, err)
	}
	if insidePath(root, output) {
		return fmt.Errorf("export bundle: output %s must not be inside knowledge directory %s", output, root)
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("export bundle: inspect knowledge directory %s: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("export bundle: knowledge path %s is not a directory", root)
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", path, walkErr)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("export bundle: discover files: %w", err)
	}
	sort.Strings(files)

	parent := filepath.Dir(output)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("export bundle: create output directory %s: %w", parent, err)
	}
	temporary, err := os.CreateTemp(parent, ".noli-export-*.zip")
	if err != nil {
		return fmt.Errorf("export bundle: create temporary output for %s: %w", output, err)
	}
	temporaryName := temporary.Name()
	defer func() {
		if closeErr := temporary.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("export bundle: close temporary output: %w", closeErr)
		}
		if removeErr := os.Remove(temporaryName); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && returnErr == nil {
			returnErr = fmt.Errorf("export bundle: clean temporary output: %w", removeErr)
		}
	}()

	archive := zip.NewWriter(temporary)
	for _, filename := range files {
		relative, err := filepath.Rel(root, filename)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			if err == nil {
				err = fmt.Errorf("path escaped knowledge directory")
			}
			_ = archive.Close()
			return fmt.Errorf("export bundle: relativize %s: %w", filename, err)
		}
		if err := addZipFile(archive, filename, filepath.ToSlash(relative)); err != nil {
			_ = archive.Close()
			return fmt.Errorf("export bundle: %w", err)
		}
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("export bundle: finalize archive: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("export bundle: sync temporary archive: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("export bundle: close temporary archive: %w", err)
	}
	if err := os.Rename(temporaryName, output); err != nil {
		return fmt.Errorf("export bundle: replace %s: %w", output, err)
	}
	return nil
}

func addZipFile(archive *zip.Writer, filename, name string) error {
	input, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open %s: %w", filename, err)
	}
	defer input.Close()
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o644)
	header.Modified = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	output, err := archive.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create archive entry %s: %w", name, err)
	}
	if _, err := io.Copy(output, input); err != nil {
		return fmt.Errorf("copy %s: %w", filename, err)
	}
	if err := input.Close(); err != nil {
		return fmt.Errorf("close %s: %w", filename, err)
	}
	return nil
}

func insidePath(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
}
