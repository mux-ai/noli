package okf

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type IndexEntry struct {
	ID          string
	Type        string
	Title       string
	Description string
}

type DirectoryIndex struct {
	Directory string
	Type      string
	Entries   []IndexEntry
}

func RenderRootIndex(title, description string, directories []DirectoryIndex, options RenderOptions) ([]byte, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("render root index: title is required")
	}
	metadata := Metadata{Type: "Navigation", Title: title, Description: strings.TrimSpace(description), Timestamp: options.Timestamp}
	frontmatter, err := renderFrontmatter(metadata)
	if err != nil {
		return nil, fmt.Errorf("render root index frontmatter: %w", err)
	}
	items := append([]DirectoryIndex(nil), directories...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Directory < items[j].Directory
	})
	var body strings.Builder
	body.WriteString("# ")
	body.WriteString(cleanHeading(title))
	body.WriteString("\n")
	if description = strings.TrimSpace(description); description != "" {
		body.WriteString("\n")
		body.WriteString(description)
		body.WriteString("\n")
	}
	if len(items) > 0 {
		body.WriteString("\n## Knowledge directories\n\n")
		for _, directory := range items {
			if err := validateDirectory(directory.Directory); err != nil {
				return nil, fmt.Errorf("render root index: %w", err)
			}
			label := strings.TrimSpace(directory.Type)
			if label == "" {
				label = directory.Directory
			}
			link := "/" + filepath.ToSlash(directory.Directory) + "/index.md"
			if options.LinkStyle == "relative" {
				link = filepath.ToSlash(filepath.Join(directory.Directory, "index.md"))
			}
			body.WriteString("- [")
			body.WriteString(escapeLinkLabel(label))
			body.WriteString("](")
			body.WriteString(link)
			body.WriteString(") — ")
			body.WriteString(strconv.Itoa(len(directory.Entries)))
			body.WriteString(" documents\n")
		}
	}
	return append(frontmatter, []byte(body.String())...), nil
}

func RenderDirectoryIndex(directory DirectoryIndex, options RenderOptions) ([]byte, error) {
	if err := validateDirectory(directory.Directory); err != nil {
		return nil, fmt.Errorf("render directory index: %w", err)
	}
	typeName := strings.TrimSpace(directory.Type)
	if typeName == "" {
		return nil, fmt.Errorf("render directory index %q: concept type is required", directory.Directory)
	}
	title := typeName
	metadata := Metadata{Type: "Navigation", Title: title, Description: "Index of " + typeName + " knowledge.", Timestamp: options.Timestamp}
	frontmatter, err := renderFrontmatter(metadata)
	if err != nil {
		return nil, fmt.Errorf("render directory index %q frontmatter: %w", directory.Directory, err)
	}
	entries := append([]IndexEntry(nil), directory.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		left, right := strings.ToLower(entries[i].Title), strings.ToLower(entries[j].Title)
		if left != right {
			return left < right
		}
		return entries[i].ID < entries[j].ID
	})
	var body strings.Builder
	body.WriteString("# ")
	body.WriteString(cleanHeading(title))
	body.WriteString("\n")
	if len(entries) > 0 {
		body.WriteString("\n")
	}
	for _, entry := range entries {
		if err := ValidateConceptID(entry.ID); err != nil {
			return nil, fmt.Errorf("render directory index %q: %w", directory.Directory, err)
		}
		label := strings.TrimSpace(entry.Title)
		if label == "" {
			label = filepath.Base(entry.ID)
		}
		link := "/" + filepath.ToSlash(entry.ID) + ".md"
		if options.LinkStyle == "relative" {
			from := filepath.FromSlash(filepath.Join(directory.Directory, "index.md"))
			target := filepath.FromSlash(entry.ID + ".md")
			relative, relErr := filepath.Rel(filepath.Dir(from), target)
			if relErr != nil {
				return nil, fmt.Errorf("render relative index link to %q: %w", entry.ID, relErr)
			}
			link = filepath.ToSlash(relative)
		}
		body.WriteString("- [")
		body.WriteString(escapeLinkLabel(label))
		body.WriteString("](")
		body.WriteString(link)
		body.WriteString(")")
		if description := strings.TrimSpace(entry.Description); description != "" {
			body.WriteString(" — ")
			body.WriteString(escapeInline(description))
		}
		body.WriteString("\n")
	}
	return append(frontmatter, []byte(body.String())...), nil
}

func validateDirectory(directory string) error {
	if directory == "" || filepath.IsAbs(directory) || strings.HasPrefix(directory, "/") || strings.Contains(directory, "\\") {
		return fmt.Errorf("unsafe knowledge directory %q", directory)
	}
	clean := filepath.Clean(filepath.FromSlash(directory))
	if clean == "." || clean == ".." || clean != filepath.FromSlash(directory) || escapesRoot(clean) {
		return fmt.Errorf("unsafe knowledge directory %q", directory)
	}
	return nil
}
