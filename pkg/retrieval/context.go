package retrieval

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// assembleContext renders the frozen context format within the rune budget:
//
//	# Context for: <query>
//
//	## Source: <id> (<type>, seed, score <n>)
//
//	<content>
//
//	## Source: <id> (<type>, distance <n> via <predecessor>, <relationship>)
//
//	<content>
//
// Complete sections are added while they fit. The first section that does
// not fit completely is truncated on a rune boundary when its header plus at
// least one content rune fits; assembly stops there and later selections are
// dropped. If even the very first source header cannot fit,
// ErrContextLimitTooSmall is returned.
func assembleContext(query string, selected []selection, maxCharacters int) (Result, error) {
	result := Result{
		Query:      query,
		Sources:    []Source{},
		Statistics: Statistics{MaxCharacters: maxCharacters},
	}
	var builder strings.Builder
	preamble := "# Context for: " + query + "\n"
	used := utf8.RuneCountInString(preamble)
	firstHeader := sectionHeader(selected[0].source)
	if used+utf8.RuneCountInString(firstHeader) > maxCharacters {
		return Result{}, fmt.Errorf("maximum characters %d: %w", maxCharacters, ErrContextLimitTooSmall)
	}
	builder.WriteString(preamble)

	for _, item := range selected {
		header := sectionHeader(item.source)
		content := strings.TrimSpace(item.record.Content)
		full := header + content + "\n"
		fullRunes := utf8.RuneCountInString(full)
		if used+fullRunes <= maxCharacters {
			builder.WriteString(full)
			used += fullRunes
			result.Sources = append(result.Sources, item.source)
			continue
		}
		// The section does not fit completely. Truncate on a rune boundary
		// when the header and at least one content rune fit; then stop.
		remaining := maxCharacters - used - utf8.RuneCountInString(header) - 1
		if remaining > 0 && content != "" {
			runes := []rune(content)
			if remaining < len(runes) {
				truncated := item.source
				truncated.Truncated = true
				builder.WriteString(header)
				builder.WriteString(string(runes[:remaining]))
				builder.WriteString("\n")
				used += utf8.RuneCountInString(header) + remaining + 1
				result.Sources = append(result.Sources, truncated)
			}
		}
		result.Statistics.Truncated = true
		break
	}
	result.Context = builder.String()
	for _, source := range result.Sources {
		if source.Seed {
			result.Statistics.SeedCount++
		} else {
			result.Statistics.GraphCount++
		}
		if source.Truncated {
			result.Statistics.Truncated = true
		}
	}
	result.Statistics.DocumentCount = len(result.Sources)
	result.Statistics.CharacterCount = utf8.RuneCountInString(result.Context)
	return result, nil
}

func sectionHeader(source Source) string {
	if source.Seed {
		return fmt.Sprintf("\n## Source: %s (%s, seed, score %d)\n\n", source.ID, source.Type, source.Score)
	}
	return fmt.Sprintf("\n## Source: %s (%s, distance %d via %s, %s)\n\n",
		source.ID, source.Type, source.Distance, source.Predecessor, source.Relationship)
}
