package okf

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"noli/internal/project"
)

type ValidationMode string

const (
	StandardMode ValidationMode = "standard"
	ProjectMode  ValidationMode = "project"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Problem struct {
	Path     string   `json:"path,omitempty"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Severity Severity `json:"severity"`
}

func (p Problem) Error() string {
	prefix := string(p.Severity)
	if p.Path != "" {
		prefix += " " + p.Path
	}
	if p.Code != "" {
		prefix += " [" + p.Code + "]"
	}
	return prefix + ": " + p.Message
}

type ValidationError struct {
	Problems []Problem
}

func (e *ValidationError) Error() string {
	if e == nil || len(e.Problems) == 0 {
		return "bundle validation failed"
	}
	parts := make([]string, len(e.Problems))
	for i, problem := range e.Problems {
		parts[i] = problem.Error()
	}
	return fmt.Sprintf("bundle validation failed (%d problems): %s", len(parts), strings.Join(parts, "; "))
}

type ValidationOptions struct {
	Profile                 *project.ProjectProfile
	UnresolvedRelations     int
	StrictRelationships     bool
	RequireBundleLog        bool
	RequireDirectoryIndexes bool
}

// ProblemsError returns an aggregated error for error-severity problems and
// ignores warnings. It is convenient for CLI exit handling.
func ProblemsError(problems []Problem) error {
	var failures []Problem
	for _, problem := range problems {
		if problem.Severity == SeverityError || problem.Severity == "" {
			if problem.Severity == "" {
				problem.Severity = SeverityError
			}
			failures = append(failures, problem)
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return &ValidationError{Problems: failures}
}

// ValidateBundle reports all validation problems that can be found without
// depending on an earlier check succeeding.
func ValidateBundle(root string, mode ValidationMode, options ValidationOptions) []Problem {
	var problems []Problem
	add := func(path, code, message string, severity Severity) {
		problems = append(problems, Problem{Path: path, Code: code, Message: message, Severity: severity})
	}
	if mode != StandardMode && mode != ProjectMode {
		add("", "invalid-mode", fmt.Sprintf("unknown validation mode %q", mode), SeverityError)
		return sortedProblems(problems)
	}

	bundle, parseErr := ParseBundle(root)
	if parseErr != nil {
		var aggregate *ParseErrors
		if errors.As(parseErr, &aggregate) {
			for _, err := range aggregate.Errors {
				add("", "parse", err.Error(), SeverityError)
			}
		} else {
			add("", "parse", parseErr.Error(), SeverityError)
		}
	}
	if bundle == nil {
		return sortedProblems(problems)
	}

	if _, exists := bundle.Documents["index"]; !exists {
		add("index.md", "missing-root-index", "bundle must contain a root index.md", SeverityError)
	}
	if _, exists := bundle.Documents["log"]; !exists {
		add("log.md", "missing-bundle-log", "bundle must contain a root log.md", SeverityError)
	}

	conceptDirectories := make(map[string]struct{})
	for _, id := range bundle.Order {
		document := bundle.Documents[id]
		if document.IsIndex {
			if !strings.EqualFold(strings.TrimSpace(document.Metadata.Type), "Navigation") {
				add(document.Path, "invalid-index-type", "index documents must have type Navigation", SeverityError)
			}
			if strings.TrimSpace(document.Body) == "" {
				add(document.Path, "empty-index", "index documents must not be empty", SeverityError)
			}
			continue
		}
		if document.IsLog {
			if !strings.EqualFold(strings.TrimSpace(document.Metadata.Type), "Bundle Log") {
				add(document.Path, "invalid-log-type", "log.md must have type Bundle Log", SeverityError)
			}
			continue
		}
		if strings.TrimSpace(document.Metadata.Type) == "" {
			add(document.Path, "missing-type", "concept document frontmatter requires a non-empty type", SeverityError)
		}
		directory := filepath.ToSlash(filepath.Dir(filepath.FromSlash(document.Path)))
		if directory != "." {
			conceptDirectories[directory] = struct{}{}
		}
	}
	for directory := range conceptDirectories {
		indexID := filepath.ToSlash(filepath.Join(directory, "index"))
		if _, exists := bundle.Documents[indexID]; !exists {
			add(filepath.ToSlash(filepath.Join(directory, "index.md")), "missing-directory-index", "concept directory must contain index.md", SeverityError)
		}
	}

	if mode == ProjectMode {
		validateProjectBundle(bundle, options, add)
	}
	return sortedProblems(problems)
}

func validateProjectBundle(bundle *ParsedBundle, options ValidationOptions, add func(string, string, string, Severity)) {
	if options.Profile == nil {
		add("", "missing-profile", "project validation requires an active project profile", SeverityError)
		return
	}
	profile := options.Profile
	if err := project.ValidateProfile(*profile); err != nil {
		add("", "invalid-profile", err.Error(), SeverityError)
		return
	}
	types := make(map[string]project.ConceptTypeConfig)
	for _, conceptType := range profile.ConceptTypes {
		types[strings.ToLower(strings.TrimSpace(conceptType.Type))] = conceptType
		for _, alias := range conceptType.Aliases {
			types[strings.ToLower(strings.TrimSpace(alias))] = conceptType
		}
	}
	seenConcepts := make(map[string]string)
	for _, id := range bundle.Order {
		document := bundle.Documents[id]
		if err := validateMetadata(document.Metadata); err != nil {
			add(document.Path, "unsafe-metadata", err.Error(), SeverityError)
		}
		for _, target := range document.Links {
			if _, exists := bundle.Documents[target]; !exists {
				add(document.Path, "unresolved-link", fmt.Sprintf("local link target %q does not exist", target), SeverityError)
			}
		}
		if document.IsIndex || document.IsLog {
			continue
		}
		config, exists := types[strings.ToLower(strings.TrimSpace(document.Metadata.Type))]
		if !exists {
			add(document.Path, "unknown-concept-type", fmt.Sprintf("type %q is not configured in the project profile", document.Metadata.Type), SeverityError)
		} else {
			expectedDirectory := filepath.ToSlash(filepath.Clean(filepath.FromSlash(config.Directory)))
			actualDirectory := filepath.ToSlash(filepath.Dir(filepath.FromSlash(document.Path)))
			if expectedDirectory != actualDirectory {
				add(document.Path, "wrong-concept-directory", fmt.Sprintf("type %q belongs in directory %q", config.Type, config.Directory), SeverityError)
			}
			for _, field := range config.RequiredFields {
				if !metadataFieldPresent(document.Metadata, field) {
					add(document.Path, "missing-required-field", fmt.Sprintf("required metadata field %q is absent or empty", field), SeverityError)
				}
			}
			sections := markdownSections(document.Body)
			for _, heading := range config.RequiredSections {
				if strings.TrimSpace(sections[strings.ToLower(strings.TrimSpace(heading))]) == "" {
					add(document.Path, "missing-required-section", fmt.Sprintf("required section %q is absent or empty", heading), SeverityError)
				}
			}
		}
		for _, field := range profile.Validation.RequiredMetadata {
			if !metadataFieldPresent(document.Metadata, field) {
				add(document.Path, "missing-project-metadata", fmt.Sprintf("project-required metadata field %q is absent or empty", field), SeverityError)
			}
		}
		confidence, ok := numericMetadata(document.Metadata.Extra, "confidence")
		if !ok {
			add(document.Path, "missing-confidence", "concept metadata requires a numeric confidence", SeverityError)
		} else if confidence < 0 || confidence > 1 {
			add(document.Path, "invalid-confidence", "confidence must be between 0 and 1", SeverityError)
		} else if confidence < profile.Extraction.MinimumConfidence {
			add(document.Path, "low-confidence", fmt.Sprintf("confidence %.3f is below project minimum %.3f", confidence, profile.Extraction.MinimumConfidence), SeverityError)
		}
		if profile.Extraction.RequireSourceEvidence || profile.Validation.RequireCitations {
			if strings.TrimSpace(markdownSections(document.Body)["citations"]) == "" {
				add(document.Path, "missing-citations", "project requires source citations", SeverityError)
			}
		}
		if strings.TrimSpace(document.Body) == "" {
			add(document.Path, "empty-document", "concept document body must not be empty", SeverityError)
		}
		key := ""
		if exists {
			key = conceptIdentityKey(document.Metadata, config)
		}
		if key != "" {
			if previous, duplicate := seenConcepts[key]; duplicate {
				add(document.Path, "duplicate-concept", fmt.Sprintf("configured identity fields duplicate %q", previous), SeverityError)
			} else {
				seenConcepts[key] = document.ID
			}
		}
	}
	if options.UnresolvedRelations > 0 {
		severity := SeverityWarning
		if options.StrictRelationships {
			severity = SeverityError
		}
		add("", "unresolved-relationships", fmt.Sprintf("%d relationships could not be resolved", options.UnresolvedRelations), severity)
	}
}

func metadataFieldPresent(metadata Metadata, field string) bool {
	value, exists := metadataFieldValue(metadata, field)
	return exists && valuePresent(value)
}

func metadataFieldValue(metadata Metadata, field string) (any, bool) {
	field = strings.TrimSpace(field)
	switch strings.ToLower(field) {
	case "type":
		return metadata.Type, true
	case "title":
		return metadata.Title, true
	case "description":
		return metadata.Description, true
	case "resource":
		return metadata.Resource, true
	case "tags":
		return metadata.Tags, true
	case "timestamp":
		return metadata.Timestamp, true
	}
	parts := strings.Split(field, ".")
	var value any = metadata.Extra
	for _, part := range parts {
		mapping, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		found := false
		for key, candidate := range mapping {
			if strings.EqualFold(key, part) {
				value = candidate
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return value, true
}

func conceptIdentityKey(metadata Metadata, config project.ConceptTypeConfig) string {
	parts := []string{strings.ToLower(strings.TrimSpace(config.Type))}
	hasValue := false
	for _, field := range config.IdentityFields {
		value, exists := metadataFieldValue(metadata, field)
		if !exists || !valuePresent(value) {
			parts = append(parts, strings.ToLower(strings.TrimSpace(field))+"=")
			continue
		}
		hasValue = true
		encoded, err := json.Marshal(value)
		if err != nil {
			encoded = []byte(fmt.Sprint(value))
		}
		parts = append(parts, strings.ToLower(strings.TrimSpace(field))+"="+normalizeComparison(string(encoded)))
	}
	if !hasValue {
		return ""
	}
	return strings.Join(parts, "\x00")
}

func valuePresent(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case []string:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func numericMetadata(metadata map[string]any, key string) (float64, bool) {
	for candidate, value := range metadata {
		if !strings.EqualFold(candidate, key) {
			continue
		}
		switch number := value.(type) {
		case float64:
			return number, true
		case float32:
			return float64(number), true
		case int:
			return float64(number), true
		case int64:
			return float64(number), true
		case uint64:
			return float64(number), true
		}
	}
	return 0, false
}

func markdownSections(body string) map[string]string {
	sections := make(map[string]string)
	current := ""
	var content strings.Builder
	inFence := false
	flush := func() {
		if current != "" {
			sections[current] = strings.TrimSpace(content.String())
		}
		content.Reset()
	}
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		}
		if !inFence {
			hashCount := 0
			for hashCount < len(line) && hashCount < 6 && line[hashCount] == '#' {
				hashCount++
			}
			if hashCount > 0 && len(line) > hashCount && line[hashCount] == ' ' {
				flush()
				current = strings.ToLower(strings.TrimSpace(line[hashCount+1:]))
				continue
			}
		}
		if current != "" {
			content.WriteString(line)
			content.WriteByte('\n')
		}
	}
	flush()
	return sections
}

func validateMetadata(metadata Metadata) error {
	if !utf8.ValidString(metadata.Type + metadata.Title + metadata.Description + metadata.Resource + metadata.Timestamp) {
		return fmt.Errorf("common metadata contains invalid UTF-8")
	}
	if strings.ContainsRune(metadata.Resource, '\x00') {
		return fmt.Errorf("resource contains a NUL byte")
	}
	if parsed, err := url.Parse(metadata.Resource); err == nil && parsed.Scheme != "" {
		scheme := strings.ToLower(parsed.Scheme)
		if scheme == "javascript" || scheme == "data" {
			return fmt.Errorf("resource uses unsafe URI scheme %q", scheme)
		}
	}
	return validateMetadataValue(metadata.Extra, "metadata")
}

func validateMetadataValue(value any, location string) error {
	switch typed := value.(type) {
	case nil, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	case float32:
		if math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) {
			return fmt.Errorf("%s contains a non-finite number", location)
		}
		return nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return fmt.Errorf("%s contains a non-finite number", location)
		}
		return nil
	case string:
		if !utf8.ValidString(typed) || strings.ContainsRune(typed, '\x00') {
			return fmt.Errorf("%s contains unsafe text", location)
		}
		return nil
	case time.Time:
		return nil
	case []string:
		for i, item := range typed {
			if err := validateMetadataValue(item, fmt.Sprintf("%s[%d]", location, i)); err != nil {
				return err
			}
		}
		return nil
	case []any:
		for i, item := range typed {
			if err := validateMetadataValue(item, fmt.Sprintf("%s[%d]", location, i)); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if strings.TrimSpace(key) == "" || strings.ContainsRune(key, '\x00') || strings.ContainsFunc(key, unicode.IsControl) {
				return fmt.Errorf("%s contains an unsafe key", location)
			}
			if err := validateMetadataValue(typed[key], location+"."+key); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("%s contains unsupported value type %T", location, value)
	}
}

func normalizeComparison(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func sortedProblems(problems []Problem) []Problem {
	sort.SliceStable(problems, func(i, j int) bool {
		if problems[i].Path != problems[j].Path {
			return problems[i].Path < problems[j].Path
		}
		if problems[i].Code != problems[j].Code {
			return problems[i].Code < problems[j].Code
		}
		if problems[i].Severity != problems[j].Severity {
			return problems[i].Severity < problems[j].Severity
		}
		return problems[i].Message < problems[j].Message
	})
	return problems
}
