package generator

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// Slug produces one safe path segment. Unicode letters and digits are
// retained; every other run becomes a single ASCII hyphen.
func Slug(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	separator := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if separator && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(r)
			separator = false
		default:
			separator = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" || result == "." || result == ".." {
		return "", fmt.Errorf("generate slug from %q: no safe letters or digits", value)
	}
	return result, nil
}

// validateConceptID rejects IDs that could escape or alias output paths.
func validateConceptID(id string) error {
	if id == "" || filepath.IsAbs(id) || strings.HasPrefix(id, "/") || strings.Contains(id, "\\") {
		return fmt.Errorf("unsafe concept ID %q", id)
	}
	clean := filepath.Clean(filepath.FromSlash(id))
	if clean == "." || clean == ".." || clean != filepath.FromSlash(id) {
		return fmt.Errorf("unsafe concept ID %q", id)
	}
	for _, part := range strings.Split(filepath.ToSlash(clean), "/") {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") {
			return fmt.Errorf("unsafe concept ID %q", id)
		}
	}
	return nil
}

// canonicalConceptID validates a supplied ID or derives one from the type
// directory and the title slug. A supplied ID must live inside the type's
// configured directory.
func canonicalConceptID(rule ConceptTypeConfig, input ConceptInput) (string, error) {
	directory := normalizedDirectory(rule.Directory)
	if supplied := strings.TrimSpace(input.ID); supplied != "" {
		if err := validateConceptID(supplied); err != nil {
			return "", err
		}
		if filepath.ToSlash(filepath.Dir(filepath.FromSlash(supplied))) != directory {
			return "", fmt.Errorf("concept ID %q must live in the %q directory configured for type %q",
				supplied, rule.Directory, rule.Type)
		}
		if base := filepath.Base(supplied); strings.EqualFold(base, "index") || strings.EqualFold(base, "log") {
			return "", fmt.Errorf("concept ID %q collides with a reserved document name", supplied)
		}
		return supplied, nil
	}
	slug, err := Slug(input.Title)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(slug, "index") || strings.EqualFold(slug, "log") {
		return "", fmt.Errorf("title %q produces a reserved document name", input.Title)
	}
	return directory + "/" + slug, nil
}

func normalizedDirectory(directory string) string {
	return strings.Trim(filepath.ToSlash(filepath.Clean(filepath.FromSlash(directory))), "/")
}
