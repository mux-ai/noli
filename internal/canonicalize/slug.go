package canonicalize

import (
	"fmt"
	"strings"
	"unicode"
)

// Slug produces one safe path segment. Unicode letters and digits are retained;
// every other run becomes a single ASCII hyphen.
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
	if strings.Contains(result, "/") || strings.Contains(result, `\`) {
		return "", fmt.Errorf("generate slug from %q: unsafe path separator", value)
	}
	return result, nil
}
