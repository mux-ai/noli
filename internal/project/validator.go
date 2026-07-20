package project

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
)

type ValidationProblem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (p ValidationProblem) String() string {
	if p.Field == "" {
		return p.Message
	}
	return p.Field + ": " + p.Message
}

// ValidationError reports all independent profile problems found in one pass.
type ValidationError struct {
	Problems []ValidationProblem
}

func (e *ValidationError) Error() string {
	if e == nil || len(e.Problems) == 0 {
		return "profile validation failed"
	}
	parts := make([]string, len(e.Problems))
	for i, problem := range e.Problems {
		parts[i] = problem.String()
	}
	return fmt.Sprintf("profile validation failed (%d problems): %s", len(parts), strings.Join(parts, "; "))
}

func ValidateProfile(profile ProjectProfile) error {
	problems := ProfileProblems(profile)
	if len(problems) == 0 {
		return nil
	}
	return &ValidationError{Problems: problems}
}

func ProfileProblems(profile ProjectProfile) []ValidationProblem {
	var problems []ValidationProblem
	add := func(field, message string) {
		problems = append(problems, ValidationProblem{Field: field, Message: message})
	}

	if profile.Version < 1 {
		add("version", "must be at least 1")
	}
	if strings.TrimSpace(profile.Project.Name) == "" {
		add("project.name", "must not be empty")
	}
	if len(profile.ConceptTypes) == 0 {
		add("concept_types", "must contain at least one concept type")
	}

	types := make(map[string]string, len(profile.ConceptTypes))
	directories := make(map[string]string, len(profile.ConceptTypes))
	for i, conceptType := range profile.ConceptTypes {
		prefix := fmt.Sprintf("concept_types[%d]", i)
		typeName := strings.TrimSpace(conceptType.Type)
		foldedType := strings.ToLower(typeName)
		if typeName == "" {
			add(prefix+".type", "must not be empty")
		} else if previous, exists := types[foldedType]; exists {
			add(prefix+".type", fmt.Sprintf("duplicates concept type %q", previous))
		} else {
			types[foldedType] = typeName
		}

		cleanDirectory, err := ValidateRelativeDirectory(conceptType.Directory)
		if err != nil {
			add(prefix+".directory", err.Error())
		} else if previous, exists := directories[strings.ToLower(cleanDirectory)]; exists && !profile.OKF.AllowDuplicateDirectories {
			add(prefix+".directory", fmt.Sprintf("duplicates directory used by %q", previous))
		} else {
			directories[strings.ToLower(cleanDirectory)] = typeName
		}

		if len(conceptType.IdentityFields) == 0 {
			add(prefix+".identity_fields", "must contain at least one field")
		}
		validateUniqueNonEmpty(&problems, prefix+".identity_fields", conceptType.IdentityFields)
		validateUniqueNonEmpty(&problems, prefix+".sections", conceptType.Sections)
		validateUniqueNonEmpty(&problems, prefix+".required_fields", conceptType.RequiredFields)
		validateUniqueNonEmpty(&problems, prefix+".required_sections", conceptType.RequiredSections)
		validateUniqueNonEmpty(&problems, prefix+".aliases", conceptType.Aliases)

		sections := make(map[string]struct{}, len(conceptType.Sections))
		for _, section := range conceptType.Sections {
			sections[strings.ToLower(strings.TrimSpace(section))] = struct{}{}
		}
		for _, required := range conceptType.RequiredSections {
			if _, ok := sections[strings.ToLower(strings.TrimSpace(required))]; !ok {
				add(prefix+".required_sections", fmt.Sprintf("%q is not listed in sections", required))
			}
		}
	}

	if profile.Extraction.MinimumConfidence < 0 || profile.Extraction.MinimumConfidence > 1 {
		add("extraction.minimum_confidence", "must be between 0 and 1")
	}
	if profile.Extraction.MaximumConceptsPerSource < 1 {
		add("extraction.maximum_concepts_per_source", "must be at least 1")
	}
	if profile.Extraction.MaximumChunkCharacters < 0 {
		add("extraction.maximum_chunk_characters", "must not be negative")
	}
	if profile.Extraction.MaximumSourceExcerpts < 0 {
		add("extraction.maximum_source_excerpts", "must not be negative")
	}
	if timeout := strings.TrimSpace(profile.Extraction.RequestTimeoutText); timeout != "" {
		if duration, err := time.ParseDuration(timeout); err != nil || duration <= 0 {
			add("extraction.request_timeout", "must be a positive Go duration such as 2m")
		}
	}
	if style := strings.TrimSpace(profile.OKF.LinkStyle); style != "" && style != "root-relative" && style != "relative" {
		add("okf.link_style", "must be root-relative or relative")
	}

	seenRelationships := make(map[string]struct{}, len(profile.Relationships))
	for i, relationship := range profile.Relationships {
		prefix := fmt.Sprintf("relationships[%d]", i)
		sourceKey := strings.ToLower(strings.TrimSpace(relationship.SourceType))
		targetKey := strings.ToLower(strings.TrimSpace(relationship.TargetType))
		relation := strings.TrimSpace(relationship.Relation)
		if sourceKey == "" {
			add(prefix+".source_type", "must not be empty")
		} else if _, exists := types[sourceKey]; !exists {
			add(prefix+".source_type", fmt.Sprintf("references unknown concept type %q", relationship.SourceType))
		}
		if targetKey == "" {
			add(prefix+".target_type", "must not be empty")
		} else if _, exists := types[targetKey]; !exists {
			add(prefix+".target_type", fmt.Sprintf("references unknown concept type %q", relationship.TargetType))
		}
		if relation == "" {
			add(prefix+".relation", "must not be empty")
		}
		key := sourceKey + "\x00" + strings.ToLower(relation) + "\x00" + targetKey
		if _, exists := seenRelationships[key]; exists {
			add(prefix, "duplicates an earlier relationship rule")
		}
		seenRelationships[key] = struct{}{}
	}

	sort.SliceStable(problems, func(i, j int) bool {
		if problems[i].Field == problems[j].Field {
			return problems[i].Message < problems[j].Message
		}
		return problems[i].Field < problems[j].Field
	})
	return problems
}

func validateUniqueNonEmpty(problems *[]ValidationProblem, field string, values []string) {
	seen := make(map[string]struct{}, len(values))
	for i, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			*problems = append(*problems, ValidationProblem{Field: fmt.Sprintf("%s[%d]", field, i), Message: "must not be empty"})
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			*problems = append(*problems, ValidationProblem{Field: field, Message: fmt.Sprintf("contains duplicate value %q", trimmed)})
		}
		seen[key] = struct{}{}
	}
}

// ValidateRelativeDirectory validates a profile's portable, slash-separated
// directory and returns its cleaned representation.
func ValidateRelativeDirectory(directory string) (string, error) {
	trimmed := strings.TrimSpace(directory)
	if trimmed == "" {
		return "", fmt.Errorf("must not be empty")
	}
	if strings.ContainsRune(trimmed, '\x00') || strings.Contains(trimmed, "\\") {
		return "", fmt.Errorf("must use safe forward-slash path components")
	}
	if strings.HasPrefix(trimmed, "/") {
		return "", fmt.Errorf("must be relative")
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("must not contain directory traversal")
	}
	for _, component := range strings.Split(cleaned, "/") {
		if component == "" || component == "." || component == ".." {
			return "", fmt.Errorf("contains an unsafe path component")
		}
	}
	return cleaned, nil
}
