package search

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// tokenize splits a value into lowercase tokens. Letters, digits, underscores,
// and interior hyphens stay inside a token; a token must contain at least one
// word character (a hyphen alone is not a token).
func tokenize(value string) []string {
	var result []string
	var token []rune
	hasWordCharacter := false
	flush := func() {
		if len(token) > 0 && hasWordCharacter {
			result = append(result, strings.ToLower(string(token)))
		}
		token = token[:0]
		hasWordCharacter = false
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			token = append(token, unicode.ToLower(r))
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				hasWordCharacter = true
			}
			continue
		}
		flush()
	}
	flush()
	return result
}

func uniqueTokens(tokens []string) []string {
	seen := make(map[string]struct{}, len(tokens))
	var result []string
	for _, token := range tokens {
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result
}

func tokenSet(tokens []string) map[string]struct{} {
	result := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		result[token] = struct{}{}
	}
	return result
}

// normalizePhrase lowercases a value and collapses all whitespace runs to
// single spaces.
func normalizePhrase(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

// metadataText flattens arbitrary custom metadata into searchable text with
// deterministic key order.
func metadataText(value any) string {
	var output strings.Builder
	appendMetadataText(&output, value)
	return output.String()
}

func appendMetadataText(output *strings.Builder, value any) {
	switch typed := value.(type) {
	case nil:
		return
	case string:
		output.WriteString(typed)
		output.WriteByte(' ')
	case []string:
		for _, item := range typed {
			appendMetadataText(output, item)
		}
	case []any:
		for _, item := range typed {
			appendMetadataText(output, item)
		}
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			output.WriteString(key)
			output.WriteByte(' ')
			appendMetadataText(output, typed[key])
		}
	default:
		fmt.Fprint(output, typed)
		output.WriteByte(' ')
	}
}
