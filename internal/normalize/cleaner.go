package normalize

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func CleanText(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("text is not valid UTF-8")
	}
	if strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("text contains a NUL byte")
	}
	value = strings.TrimPrefix(value, "\ufeff")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))
	consecutiveBlank := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			consecutiveBlank++
			if consecutiveBlank > 2 {
				continue
			}
			line = ""
		} else {
			consecutiveBlank = 0
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n")), nil
}

func CollapseWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
