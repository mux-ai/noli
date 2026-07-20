package extract

import (
	"fmt"
	"sort"
	"strings"

	"noli/internal/llm"
	"noli/internal/project"
)

// ConceptResponse is the only accepted top-level extraction response shape.
type ConceptResponse struct {
	Concepts []ConceptDraft `json:"concepts"`
}

func DecodeConceptResponse(data []byte) (ConceptResponse, error) {
	var response ConceptResponse
	if err := llm.DecodeStrictJSON(data, &response); err != nil {
		return ConceptResponse{}, fmt.Errorf("decode concept response: %w", err)
	}
	if response.Concepts == nil {
		return ConceptResponse{}, fmt.Errorf("decode concept response: required field concepts is absent")
	}
	return response, nil
}

type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	if e == nil || len(e.Problems) == 0 {
		return ""
	}
	return "concept response validation failed: " + strings.Join(e.Problems, "; ")
}

func ValidateConcepts(concepts []ConceptDraft, profile project.ProjectProfile, sourceIDs map[string]struct{}) error {
	allowedTypes := make(map[string]project.ConceptTypeConfig, len(profile.ConceptTypes))
	for _, conceptType := range profile.ConceptTypes {
		allowedTypes[normalizeName(conceptType.Type)] = conceptType
		for _, alias := range conceptType.Aliases {
			allowedTypes[normalizeName(alias)] = conceptType
		}
	}
	problems := make([]string, 0)
	seenIDs := make(map[string]int)
	for i, concept := range concepts {
		label := fmt.Sprintf("concepts[%d]", i)
		if strings.TrimSpace(concept.TemporaryID) == "" {
			problems = append(problems, label+".temporary_id is required")
		} else if previous, exists := seenIDs[concept.TemporaryID]; exists {
			problems = append(problems, fmt.Sprintf("%s.temporary_id duplicates concepts[%d]", label, previous))
		} else {
			seenIDs[concept.TemporaryID] = i
		}
		conceptType, validType := allowedTypes[normalizeName(concept.Type)]
		if strings.TrimSpace(concept.Type) == "" {
			problems = append(problems, label+".type is required")
		} else if !validType {
			problems = append(problems, fmt.Sprintf("%s.type %q is not in the project profile", label, concept.Type))
		}
		if strings.TrimSpace(concept.Title) == "" {
			problems = append(problems, label+".title is required")
		}
		if strings.TrimSpace(concept.Description) == "" {
			problems = append(problems, label+".description is required")
		}
		if concept.Confidence < 0 || concept.Confidence > 1 {
			problems = append(problems, fmt.Sprintf("%s.confidence must be between 0 and 1", label))
		}
		seenHeadings := make(map[string]int)
		for sectionIndex, section := range concept.Sections {
			sectionLabel := fmt.Sprintf("%s.sections[%d]", label, sectionIndex)
			heading := normalizeName(section.Heading)
			if heading == "" {
				problems = append(problems, sectionLabel+".heading is required")
			} else if previous, exists := seenHeadings[heading]; exists {
				problems = append(problems, fmt.Sprintf("%s.heading duplicates sections[%d]", sectionLabel, previous))
			} else {
				seenHeadings[heading] = sectionIndex
			}
			if strings.TrimSpace(section.Content) == "" {
				problems = append(problems, sectionLabel+".content is required")
			}
		}
		for relationIndex, relation := range concept.Relations {
			relationLabel := fmt.Sprintf("%s.relations[%d]", label, relationIndex)
			if strings.TrimSpace(relation.Predicate) == "" {
				problems = append(problems, relationLabel+".predicate is required")
			}
			if strings.TrimSpace(relation.TargetReference) == "" {
				problems = append(problems, relationLabel+".target_reference is required")
			}
			if relation.Confidence < 0 || relation.Confidence > 1 {
				problems = append(problems, relationLabel+".confidence must be between 0 and 1")
			}
		}
		for citationIndex, citation := range concept.Citations {
			citationLabel := fmt.Sprintf("%s.citations[%d]", label, citationIndex)
			if strings.TrimSpace(citation.SourceID) == "" {
				problems = append(problems, citationLabel+".source_id is required")
			} else if sourceIDs != nil {
				if _, exists := sourceIDs[citation.SourceID]; !exists {
					problems = append(problems, fmt.Sprintf("%s.source_id %q is unknown", citationLabel, citation.SourceID))
				}
			}
			if citation.Page < 0 || citation.StartLine < 0 || citation.EndLine < 0 {
				problems = append(problems, citationLabel+" locations must not be negative")
			}
			if citation.StartLine > 0 && citation.EndLine > 0 && citation.EndLine < citation.StartLine {
				problems = append(problems, citationLabel+".end_line precedes start_line")
			}
		}
		if profile.Extraction.RequireSourceEvidence && len(concept.Citations) == 0 {
			problems = append(problems, label+".citations is required by the project profile")
		}
		if validType {
			for _, field := range conceptType.RequiredFields {
				if !hasConceptField(concept, field) {
					problems = append(problems, fmt.Sprintf("%s is missing required field %q", label, field))
				}
			}
			for _, requiredSection := range conceptType.RequiredSections {
				if _, exists := seenHeadings[normalizeName(requiredSection)]; !exists {
					problems = append(problems, fmt.Sprintf("%s is missing required section %q", label, requiredSection))
				}
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return &ValidationError{Problems: problems}
}

func hasConceptField(concept ConceptDraft, field string) bool {
	switch normalizeName(field) {
	case "temporary id", "temporary_id":
		return strings.TrimSpace(concept.TemporaryID) != ""
	case "type":
		return strings.TrimSpace(concept.Type) != ""
	case "title":
		return strings.TrimSpace(concept.Title) != ""
	case "description":
		return strings.TrimSpace(concept.Description) != ""
	case "resource":
		return strings.TrimSpace(concept.Resource) != ""
	case "tags":
		return len(concept.Tags) > 0
	case "sections":
		return len(concept.Sections) > 0
	case "citations":
		return len(concept.Citations) > 0
	}
	for key, value := range concept.Attributes {
		if normalizeName(key) == normalizeName(field) && !emptyValue(value) {
			return true
		}
	}
	return false
}

func emptyValue(value any) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case []string:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func normalizeName(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}
