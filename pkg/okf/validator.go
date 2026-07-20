package okf

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Validation problem codes (docs/PROTOCOL.md section 8). The namespace is
// extensible; parse problems reuse the parser codes.
const (
	CodeMissingType       = "MISSING_TYPE"
	CodeUnknownType       = "UNKNOWN_TYPE"
	CodeBrokenLink        = "BROKEN_LINK"
	CodeMissingIndex      = "MISSING_INDEX"
	CodeInvalidLogHeading = "INVALID_LOG_HEADING"
	CodeEmptyIndex        = "EMPTY_INDEX"
	CodeEmptyDocument     = "EMPTY_DOCUMENT"
	CodeMissingConfidence = "MISSING_CONFIDENCE"
	CodeInvalidConfidence = "INVALID_CONFIDENCE"
	CodeLowConfidence     = "LOW_CONFIDENCE"
	CodeMissingMetadata   = "MISSING_METADATA"
	CodeMissingSection    = "MISSING_SECTION"
	CodeMissingCitation   = "MISSING_CITATION"
	CodeWrongDirectory    = "WRONG_DIRECTORY"
	CodeDuplicateConcept  = "DUPLICATE_CONCEPT"
	CodeUnsafeMetadata    = "UNSAFE_METADATA"
)

// Problem is one validation finding.
type Problem struct {
	Code string `json:"code"`
	// Document is the document ID, or "" for bundle-level problems.
	Document string `json:"document"`
	Message  string `json:"message"`
}

// ValidationReport aggregates all findings. Errors and Warnings are each
// sorted by (Document, Code, Message) and are never nil.
type ValidationReport struct {
	Valid    bool      `json:"valid"`
	Errors   []Problem `json:"errors"`
	Warnings []Problem `json:"warnings"`
}

// ConceptTypeRule is one configured concept type. It mirrors the noli.yaml
// concept_types entries without importing the config package.
type ConceptTypeRule struct {
	// Type is the canonical type name; Aliases match case-insensitively.
	Type    string
	Aliases []string
	// Directory is the root-relative slash directory the type belongs in.
	Directory string
	// RequiredMetadata lists metadata fields (dot paths allowed) that must
	// be present and non-empty.
	RequiredMetadata []string
	// RequiredSections lists Markdown headings that must exist and be
	// non-empty (case-insensitive).
	RequiredSections []string
	// IdentityFields define concept identity for duplicate detection.
	IdentityFields []string
}

// ProjectRules are config-derived validation rules. They are built by the
// config layer; pkg/okf never imports it.
type ProjectRules struct {
	ConceptTypes []ConceptTypeRule
	// RequiredMetadata applies to every concept document.
	RequiredMetadata []string
	// RequireConfidence requires a numeric confidence on every concept.
	RequireConfidence bool
	// MinimumConfidence, when above zero, is the lowest accepted confidence.
	MinimumConfidence float64
	// RequireCitations requires a non-empty "Citations" section.
	RequireCitations bool
}

// ValidationOptions configures validation. Standard checks always run;
// Project adds config-derived project checks.
type ValidationOptions struct {
	// RequireConfidence requires a numeric confidence field on every concept
	// document. When false, confidence is validated only when present.
	RequireConfidence bool
	// Project, when non-nil, enables project validation.
	Project *ProjectRules
	// Parse bounds and exclusions forwarded to the parser.
	Parse ParseOptions
}

// Validate reports every practical problem in the bundle below root without
// stopping at the first failure.
func Validate(root string, options ValidationOptions) ValidationReport {
	report := ValidationReport{Errors: []Problem{}, Warnings: []Problem{}}
	addError := func(code, document, message string) {
		report.Errors = append(report.Errors, Problem{Code: code, Document: document, Message: message})
	}
	addWarning := func(code, document, message string) {
		report.Warnings = append(report.Warnings, Problem{Code: code, Document: document, Message: message})
	}

	bundle, err := ParseBundle(root, options.Parse)
	if err != nil {
		var aggregate *ParseErrors
		if errors.As(err, &aggregate) {
			for _, problem := range aggregate.Problems {
				addError(problem.Code, problem.Document, problem.Message)
			}
		} else {
			addError(CodeParseError, "", err.Error())
		}
	}
	if bundle == nil {
		return finishReport(report)
	}

	// OKF v0.1 section 9 forbids consumers from rejecting a bundle because
	// an index.md is missing, so structural gaps are warnings in standard
	// mode. Project mode may escalate them as opt-in local policy.
	structural := addWarning
	if options.Project != nil {
		structural = addError
	}
	if _, exists := bundle.Documents["index"]; !exists {
		structural(CodeMissingIndex, "index", "bundle has no root index.md")
	}

	requireConfidence := options.RequireConfidence
	minimumConfidence := 0.0
	if options.Project != nil {
		requireConfidence = requireConfidence || options.Project.RequireConfidence
		minimumConfidence = options.Project.MinimumConfidence
	}

	conceptDirectories := make(map[string]struct{})
	for _, id := range bundle.Order {
		document := bundle.Documents[id]

		// Every broken internal link is reported, but section 9 forbids
		// rejecting a bundle for broken cross-links: warning in standard
		// mode, error only under opt-in project rules.
		for _, link := range document.Links {
			if _, exists := bundle.Documents[link.Target]; !exists {
				structural(CodeBrokenLink, id, fmt.Sprintf("link target %q does not exist", link.Target))
			}
		}

		// Reserved documents (index.md, log.md) carry no required
		// frontmatter under sections 6 and 7; only their body structure
		// matters, and a malformed body must not fail the bundle.
		if document.IsIndex {
			if strings.TrimSpace(document.Body) == "" {
				addWarning(CodeEmptyIndex, id, "index document is empty")
			}
			continue
		}
		if document.IsLog {
			validateLogHeadings(document, addWarning)
			continue
		}

		if strings.TrimSpace(document.Metadata.Type) == "" {
			addError(CodeMissingType, id, "concept document frontmatter requires a non-empty type")
		}
		if strings.TrimSpace(document.Body) == "" {
			addWarning(CodeEmptyDocument, id, "concept document body is empty")
		}
		validateConfidence(document, requireConfidence, minimumConfidence, addError)

		directory := filepath.ToSlash(filepath.Dir(filepath.FromSlash(document.Path)))
		if directory != "." {
			conceptDirectories[directory] = struct{}{}
		}
	}
	for directory := range conceptDirectories {
		indexID := directory + "/index"
		if _, exists := bundle.Documents[indexID]; !exists {
			structural(CodeMissingIndex, indexID, "concept directory has no index.md")
		}
	}
	if options.Project != nil {
		validateProjectRules(bundle, options.Project, addError)
	}
	return finishReport(report)
}

// validateProjectRules applies config-derived rules to every concept
// document. Index and log documents are exempt.
func validateProjectRules(bundle *ParsedBundle, rules *ProjectRules, addError func(code, document, message string)) {
	types := make(map[string]ConceptTypeRule, len(rules.ConceptTypes))
	for _, rule := range rules.ConceptTypes {
		types[strings.ToLower(strings.TrimSpace(rule.Type))] = rule
		for _, alias := range rule.Aliases {
			types[strings.ToLower(strings.TrimSpace(alias))] = rule
		}
	}
	seenConcepts := make(map[string]string)
	for _, id := range bundle.Order {
		document := bundle.Documents[id]
		if err := validateMetadataSafety(document.Metadata); err != nil {
			addError(CodeUnsafeMetadata, id, err.Error())
		}
		if document.IsIndex || document.IsLog {
			continue
		}
		sections := markdownSections(document.Body)

		rule, known := types[strings.ToLower(strings.TrimSpace(document.Metadata.Type))]
		if !known {
			addError(CodeUnknownType, id,
				fmt.Sprintf("type %q is not configured in noli.yaml", document.Metadata.Type))
		} else {
			expected := strings.Trim(filepath.ToSlash(filepath.Clean(filepath.FromSlash(rule.Directory))), "/")
			actual := filepath.ToSlash(filepath.Dir(filepath.FromSlash(document.Path)))
			if actual == "." {
				actual = ""
			}
			if expected != actual {
				addError(CodeWrongDirectory, id,
					fmt.Sprintf("type %q belongs in directory %q", rule.Type, rule.Directory))
			}
			for _, field := range rule.RequiredMetadata {
				if !metadataFieldPresent(document.Metadata, field) {
					addError(CodeMissingMetadata, id,
						fmt.Sprintf("required metadata field %q is absent or empty", field))
				}
			}
			for _, heading := range rule.RequiredSections {
				if strings.TrimSpace(sections[strings.ToLower(strings.TrimSpace(heading))]) == "" {
					addError(CodeMissingSection, id,
						fmt.Sprintf("required section %q is absent or empty", heading))
				}
			}
			if key := conceptIdentityKey(document.Metadata, rule); key != "" {
				if previous, duplicate := seenConcepts[key]; duplicate {
					addError(CodeDuplicateConcept, id,
						fmt.Sprintf("configured identity fields duplicate %q", previous))
				} else {
					seenConcepts[key] = id
				}
			}
		}
		for _, field := range rules.RequiredMetadata {
			if !metadataFieldPresent(document.Metadata, field) {
				addError(CodeMissingMetadata, id,
					fmt.Sprintf("project-required metadata field %q is absent or empty", field))
			}
		}
		if rules.RequireCitations && strings.TrimSpace(sections["citations"]) == "" {
			addError(CodeMissingCitation, id, "project requires a non-empty Citations section")
		}
	}
}

// isoDateHeading matches the ISO 8601 date headings required by OKF v0.1
// section 7 for log entries.
var isoDateHeading = regexp.MustCompile(`^#{2,6}\s+\d{4}-\d{2}-\d{2}\s*$`)

// logEntryHeading matches level-2-or-deeper headings, which carry log entry
// dates. A level-1 heading is the document title, not an entry.
var logEntryHeading = regexp.MustCompile(`^#{2,6}\s+\S`)

// validateLogHeadings reports log date headings that are not ISO 8601
// YYYY-MM-DD. Malformed logs never fail a bundle; section 9 does not permit
// rejecting for them.
func validateLogHeadings(document Document, addWarning func(code, document, message string)) {
	inFence := false
	for _, line := range strings.Split(document.Body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || !logEntryHeading.MatchString(trimmed) {
			continue
		}
		if !isoDateHeading.MatchString(trimmed) {
			addWarning(CodeInvalidLogHeading, document.ID,
				fmt.Sprintf("log heading %q is not an ISO 8601 YYYY-MM-DD date", trimmed))
		}
	}
}

// validateConfidence checks the optional numeric confidence field. Presence
// is required only when the caller demands it; a present value must be a
// number in [0, 1] and, when a positive minimum is configured, at least that
// minimum.
func validateConfidence(document Document, required bool, minimum float64, addError func(code, document, message string)) {
	value, present := metadataValue(document.Metadata.Extra, "confidence")
	if !present {
		if required {
			addError(CodeMissingConfidence, document.ID, "concept metadata requires a numeric confidence")
		}
		return
	}
	number, numeric := asNumber(value)
	if !numeric {
		addError(CodeInvalidConfidence, document.ID, "confidence must be a number")
		return
	}
	if number < 0 || number > 1 {
		addError(CodeInvalidConfidence, document.ID, "confidence must be between 0 and 1")
		return
	}
	if minimum > 0 && number < minimum {
		addError(CodeLowConfidence, document.ID,
			fmt.Sprintf("confidence %.3f is below the project minimum %.3f", number, minimum))
	}
}

// metadataFieldPresent reports whether a common or custom metadata field
// (dot paths allowed for nested custom metadata) is present and non-empty.
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
		keys := make([]string, 0, len(mapping))
		for key := range mapping {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		found := false
		for _, key := range keys {
			if strings.EqualFold(key, part) {
				value = mapping[key]
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

// conceptIdentityKey builds a deterministic identity key from the rule's
// identity fields, or "" when no identity field carries a value.
func conceptIdentityKey(metadata Metadata, rule ConceptTypeRule) string {
	parts := []string{strings.ToLower(strings.TrimSpace(rule.Type))}
	hasValue := false
	for _, field := range rule.IdentityFields {
		value, exists := metadataFieldValue(metadata, field)
		if !exists || !valuePresent(value) {
			parts = append(parts, strings.ToLower(strings.TrimSpace(field))+"=")
			continue
		}
		hasValue = true
		encoded, err := json.Marshal(value)
		if err != nil {
			encoded = fmt.Append(nil, value)
		}
		parts = append(parts, strings.ToLower(strings.TrimSpace(field))+"="+normalizeComparison(string(encoded)))
	}
	if !hasValue {
		return ""
	}
	return strings.Join(parts, "\x00")
}

func normalizeComparison(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

// markdownSections maps lowercase headings to their content, skipping fenced
// code blocks.
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

// validateMetadataSafety rejects unsafe metadata values: invalid UTF-8, NUL
// bytes, unsafe resource URI schemes, non-finite numbers, control-character
// keys, and unsupported value types.
func validateMetadataSafety(metadata Metadata) error {
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

func metadataValue(metadata map[string]any, key string) (any, bool) {
	for candidate, value := range metadata {
		if strings.EqualFold(candidate, key) {
			return value, true
		}
	}
	return nil, false
}

func asNumber(value any) (float64, bool) {
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
	default:
		return 0, false
	}
}

func finishReport(report ValidationReport) ValidationReport {
	sortProblems(report.Errors)
	sortProblems(report.Warnings)
	report.Valid = len(report.Errors) == 0
	return report
}

func sortProblems(problems []Problem) {
	sort.SliceStable(problems, func(i, j int) bool {
		if problems[i].Document != problems[j].Document {
			return problems[i].Document < problems[j].Document
		}
		if problems[i].Code != problems[j].Code {
			return problems[i].Code < problems[j].Code
		}
		return problems[i].Message < problems[j].Message
	})
}
