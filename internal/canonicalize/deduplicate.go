package canonicalize

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"noli/internal/extract"
	"noli/internal/project"
)

func FindDuplicateCandidates(profile project.ProjectProfile, drafts []extract.ConceptDraft) []DuplicateGroup {
	configs := conceptTypeConfigs(profile)
	groups := make(map[string][]string)
	for _, draft := range drafts {
		config, exists := configs[normalizeComparison(draft.Type)]
		if !exists {
			continue
		}
		key := duplicateKey(config, draft)
		groups[key] = append(groups[key], draft.TemporaryID)
	}
	result := make([]DuplicateGroup, 0)
	for key, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		result = append(result, DuplicateGroup{Key: key, TemporaryIDs: ids})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

func duplicateKey(config project.ConceptTypeConfig, draft extract.ConceptDraft) string {
	parts := []string{normalizeComparison(config.Type)}
	missing := false
	for _, field := range config.IdentityFields {
		value, ok := draftField(draft, field)
		if !ok || normalizeAnyComparison(value) == "" {
			missing = true
			continue
		}
		parts = append(parts, normalizeComparison(field)+"="+normalizeAnyComparison(value))
	}
	if len(parts) == 1 || missing {
		parts = append(parts, "title="+normalizeComparison(draft.Title))
		parts = append(parts, "content="+draftIdentityFingerprint(draft))
	}
	return strings.Join(parts, "|")
}

func draftIdentityFingerprint(draft extract.ConceptDraft) string {
	identityCopy := struct {
		Title       string                 `json:"title"`
		Description string                 `json:"description"`
		Resource    string                 `json:"resource"`
		Attributes  map[string]any         `json:"attributes"`
		Sections    []extract.SectionDraft `json:"sections"`
	}{
		Title:       normalizeComparison(draft.Title),
		Description: normalizeComparison(draft.Description),
		Resource:    normalizeComparison(draft.Resource),
		Attributes:  draft.Attributes,
		Sections:    draft.Sections,
	}
	data, _ := json.Marshal(identityCopy)
	return strings.ToLower(string(data))
}

func conceptTypeConfigs(profile project.ProjectProfile) map[string]project.ConceptTypeConfig {
	result := make(map[string]project.ConceptTypeConfig, len(profile.ConceptTypes))
	for _, config := range profile.ConceptTypes {
		result[normalizeComparison(config.Type)] = config
		for _, alias := range config.Aliases {
			result[normalizeComparison(alias)] = config
		}
	}
	return result
}

func draftField(draft extract.ConceptDraft, field string) (any, bool) {
	switch normalizeComparison(field) {
	case "temporary id", "temporary_id":
		return draft.TemporaryID, strings.TrimSpace(draft.TemporaryID) != ""
	case "type":
		return draft.Type, strings.TrimSpace(draft.Type) != ""
	case "title", "name":
		return draft.Title, strings.TrimSpace(draft.Title) != ""
	case "description":
		return draft.Description, strings.TrimSpace(draft.Description) != ""
	case "resource":
		return draft.Resource, strings.TrimSpace(draft.Resource) != ""
	}
	for key, value := range draft.Attributes {
		if normalizeComparison(key) == normalizeComparison(field) {
			return value, value != nil
		}
	}
	return nil, false
}

func canonicalField(concept CanonicalConcept, field string) (any, bool) {
	switch normalizeComparison(field) {
	case "type":
		return concept.Type, strings.TrimSpace(concept.Type) != ""
	case "title", "name":
		return concept.Title, strings.TrimSpace(concept.Title) != ""
	case "description":
		return concept.Description, strings.TrimSpace(concept.Description) != ""
	case "resource":
		return concept.Resource, strings.TrimSpace(concept.Resource) != ""
	}
	for key, value := range concept.Attributes {
		if normalizeComparison(key) == normalizeComparison(field) {
			return value, value != nil
		}
	}
	return nil, false
}

func normalizeAnyComparison(value any) string {
	if stringValue, ok := value.(string); ok {
		return normalizeComparison(stringValue)
	}
	data, err := json.Marshal(normalizeValue(value))
	if err != nil {
		return fmt.Sprint(value)
	}
	return strings.ToLower(string(data))
}

func normalizeComparison(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}
