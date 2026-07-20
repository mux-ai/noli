package canonicalize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"noli/internal/extract"
	"noli/internal/project"
)

func Canonicalize(profile project.ProjectProfile, drafts []extract.ConceptDraft) ([]CanonicalConcept, error) {
	configs := conceptTypeConfigs(profile)
	sortedDrafts := append([]extract.ConceptDraft(nil), drafts...)
	sort.SliceStable(sortedDrafts, func(i, j int) bool { return draftSortKey(sortedDrafts[i]) < draftSortKey(sortedDrafts[j]) })
	groups := make(map[string][]extract.ConceptDraft)
	for _, draft := range sortedDrafts {
		config, exists := configs[normalizeComparison(draft.Type)]
		if !exists {
			return nil, fmt.Errorf("canonicalize concept %q: unknown concept type %q", draft.TemporaryID, draft.Type)
		}
		groups[duplicateKey(config, draft)] = append(groups[duplicateKey(config, draft)], draft)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	concepts := make([]CanonicalConcept, 0, len(keys))
	for _, key := range keys {
		config := configs[normalizeComparison(groups[key][0].Type)]
		concept, err := MergeDrafts(config, groups[key])
		if err != nil {
			return nil, fmt.Errorf("canonicalize duplicate group %q: %w", key, err)
		}
		concepts = append(concepts, concept)
	}
	if err := AssignPermanentIDs(profile, concepts); err != nil {
		return nil, fmt.Errorf("canonicalize IDs: %w", err)
	}
	sort.Slice(concepts, func(i, j int) bool { return concepts[i].ID < concepts[j].ID })
	return concepts, nil
}

func MergeDrafts(config project.ConceptTypeConfig, drafts []extract.ConceptDraft) (CanonicalConcept, error) {
	if len(drafts) == 0 {
		return CanonicalConcept{}, fmt.Errorf("merge drafts: at least one draft is required")
	}
	sorted := append([]extract.ConceptDraft(nil), drafts...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Confidence != sorted[j].Confidence {
			return sorted[i].Confidence > sorted[j].Confidence
		}
		return draftSortKey(sorted[i]) < draftSortKey(sorted[j])
	})
	primary := sorted[0]
	concept := CanonicalConcept{
		Directory:   path.Clean(strings.TrimSpace(config.Directory)),
		Type:        normalizeWhitespace(config.Type),
		Title:       normalizeWhitespace(primary.Title),
		Description: normalizeWhitespace(primary.Description),
		Resource:    normalizeWhitespace(primary.Resource),
		Attributes:  make(map[string]any),
		Confidence:  primary.Confidence,
		Conflicts:   make(map[string][]any),
	}
	for _, draft := range sorted {
		concept.SourceDraftIDs = appendUniqueString(concept.SourceDraftIDs, strings.TrimSpace(draft.TemporaryID), false)
		if normalized := normalizeWhitespace(draft.Title); normalized != "" && normalized != concept.Title {
			concept.Aliases = appendUniqueString(concept.Aliases, normalized, true)
			addConflict(concept.Conflicts, "title", concept.Title, normalized)
		}
		if normalized := normalizeWhitespace(draft.Description); normalized != "" && normalized != concept.Description {
			addConflict(concept.Conflicts, "description", concept.Description, normalized)
		}
		if normalized := normalizeWhitespace(draft.Resource); normalized != "" && normalized != concept.Resource {
			addConflict(concept.Conflicts, "resource", concept.Resource, normalized)
		}
		for _, tag := range draft.Tags {
			tag = normalizeComparison(tag)
			if tag != "" {
				concept.Tags = appendUniqueString(concept.Tags, tag, true)
			}
		}
		mergeAttributes(&concept, draft.Attributes)
		mergeSections(&concept, draft.Sections)
		for _, relation := range draft.Relations {
			relation.Predicate = normalizeWhitespace(relation.Predicate)
			relation.TargetReference = normalizeWhitespace(relation.TargetReference)
			relation.Evidence = normalizeWhitespace(relation.Evidence)
			if !containsRelation(concept.Relations, relation) {
				concept.Relations = append(concept.Relations, relation)
			}
		}
		for _, citation := range draft.Citations {
			citation.SourceID = strings.TrimSpace(citation.SourceID)
			citation.URI = strings.TrimSpace(citation.URI)
			citation.Evidence = normalizeWhitespace(citation.Evidence)
			if !containsCitation(concept.Citations, citation) {
				concept.Citations = append(concept.Citations, citation)
			}
		}
	}
	aliasesFromAttributes(&concept)
	sort.Strings(concept.Tags)
	sort.Strings(concept.Aliases)
	sort.Strings(concept.SourceDraftIDs)
	sort.Slice(concept.Relations, func(i, j int) bool { return relationKey(concept.Relations[i]) < relationKey(concept.Relations[j]) })
	sort.Slice(concept.Citations, func(i, j int) bool { return citationKey(concept.Citations[i]) < citationKey(concept.Citations[j]) })
	sortSections(config, concept.Sections)
	if len(concept.Attributes) == 0 {
		concept.Attributes = nil
	}
	if len(concept.Conflicts) == 0 {
		concept.Conflicts = nil
	} else {
		for key := range concept.Conflicts {
			sort.Slice(concept.Conflicts[key], func(i, j int) bool {
				return stableValue(concept.Conflicts[key][i]) < stableValue(concept.Conflicts[key][j])
			})
		}
	}
	return concept, nil
}

func AssignPermanentIDs(profile project.ProjectProfile, concepts []CanonicalConcept) error {
	configs := conceptTypeConfigs(profile)
	type idCandidate struct {
		index        int
		base         string
		withIdentity string
		stable       string
	}
	byBase := make(map[string][]idCandidate)
	for index := range concepts {
		config, exists := configs[normalizeComparison(concepts[index].Type)]
		if !exists {
			return fmt.Errorf("assign ID for %q: unknown concept type %q", concepts[index].Title, concepts[index].Type)
		}
		directory := path.Clean(strings.TrimSpace(config.Directory))
		if directory == "." || directory == "" || strings.HasPrefix(directory, "/") || directory == ".." || strings.HasPrefix(directory, "../") {
			return fmt.Errorf("assign ID for %q: unsafe directory %q", concepts[index].Title, config.Directory)
		}
		base, err := Slug(concepts[index].Title)
		if err != nil {
			return fmt.Errorf("assign ID for %q: %w", concepts[index].Title, err)
		}
		suffixes := make([]string, 0)
		for _, identityField := range config.IdentityFields {
			if normalizeComparison(identityField) == "title" || normalizeComparison(identityField) == "name" {
				continue
			}
			value, ok := canonicalField(concepts[index], identityField)
			if !ok {
				continue
			}
			suffix, err := Slug(fmt.Sprint(value))
			if err == nil && suffix != base {
				suffixes = append(suffixes, suffix)
			}
		}
		withIdentity := base
		if len(suffixes) > 0 {
			withIdentity += "-" + strings.Join(suffixes, "-")
		}
		key := directory + "/" + base
		byBase[key] = append(byBase[key], idCandidate{index: index, base: key, withIdentity: directory + "/" + withIdentity, stable: conceptSortKey(concepts[index])})
		concepts[index].Directory = directory
	}
	used := make(map[string]struct{})
	baseKeys := make([]string, 0, len(byBase))
	for key := range byBase {
		baseKeys = append(baseKeys, key)
	}
	sort.Strings(baseKeys)
	for _, key := range baseKeys {
		candidates := byBase[key]
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].stable < candidates[j].stable })
		if len(candidates) == 1 {
			concepts[candidates[0].index].ID = uniqueID(candidates[0].base, used)
			continue
		}
		identityCounts := make(map[string]int)
		for _, candidate := range candidates {
			identityCounts[candidate.withIdentity]++
		}
		for _, candidate := range candidates {
			preferred := candidate.withIdentity
			if preferred == candidate.base || identityCounts[preferred] > 1 {
				preferred = candidate.base
			}
			concepts[candidate.index].ID = uniqueID(preferred, used)
		}
	}
	return nil
}

func uniqueID(preferred string, used map[string]struct{}) string {
	if _, exists := used[preferred]; !exists {
		used[preferred] = struct{}{}
		return preferred
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", preferred, suffix)
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}

func mergeAttributes(concept *CanonicalConcept, attributes map[string]any) {
	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		value := normalizeValue(attributes[key])
		actualKey := normalizedKey
		for existingKey := range concept.Attributes {
			if normalizeComparison(existingKey) == normalizeComparison(normalizedKey) {
				actualKey = existingKey
				break
			}
		}
		existing, exists := concept.Attributes[actualKey]
		if !exists {
			concept.Attributes[actualKey] = value
			continue
		}
		if !compatibleValue(existing, value) {
			addConflict(concept.Conflicts, "attributes."+actualKey, existing, value)
		}
	}
}

func compatibleValue(left, right any) bool {
	leftString, leftIsString := left.(string)
	rightString, rightIsString := right.(string)
	if leftIsString && rightIsString {
		return normalizeComparison(leftString) == normalizeComparison(rightString)
	}
	return stableValue(left) == stableValue(right)
}

func mergeSections(concept *CanonicalConcept, sections []extract.SectionDraft) {
	for _, section := range sections {
		section.Heading = normalizeWhitespace(section.Heading)
		section.Content = normalizeMarkdown(section.Content)
		if section.Heading == "" || section.Content == "" {
			continue
		}
		found := -1
		for i := range concept.Sections {
			if normalizeComparison(concept.Sections[i].Heading) == normalizeComparison(section.Heading) {
				found = i
				break
			}
		}
		if found < 0 {
			concept.Sections = append(concept.Sections, section)
			continue
		}
		if concept.Sections[found].Content != section.Content {
			addConflict(concept.Conflicts, "sections."+concept.Sections[found].Heading, concept.Sections[found].Content, section.Content)
		}
	}
}

func sortSections(config project.ConceptTypeConfig, sections []extract.SectionDraft) {
	order := make(map[string]int, len(config.Sections))
	for index, heading := range config.Sections {
		order[normalizeComparison(heading)] = index
	}
	sort.SliceStable(sections, func(i, j int) bool {
		iOrder, iExists := order[normalizeComparison(sections[i].Heading)]
		jOrder, jExists := order[normalizeComparison(sections[j].Heading)]
		if iExists != jExists {
			return iExists
		}
		if iExists && iOrder != jOrder {
			return iOrder < jOrder
		}
		return normalizeComparison(sections[i].Heading) < normalizeComparison(sections[j].Heading)
	})
}

func aliasesFromAttributes(concept *CanonicalConcept) {
	for key, value := range concept.Attributes {
		if normalizeComparison(key) != "alias" && normalizeComparison(key) != "aliases" {
			continue
		}
		switch typed := value.(type) {
		case string:
			concept.Aliases = appendUniqueString(concept.Aliases, normalizeWhitespace(typed), true)
		case []string:
			for _, alias := range typed {
				concept.Aliases = appendUniqueString(concept.Aliases, normalizeWhitespace(alias), true)
			}
		case []any:
			for _, alias := range typed {
				if stringAlias, ok := alias.(string); ok {
					concept.Aliases = appendUniqueString(concept.Aliases, normalizeWhitespace(stringAlias), true)
				}
			}
		}
	}
}

func addConflict(conflicts map[string][]any, key string, values ...any) {
	for _, value := range values {
		if value == nil || stableValue(value) == `""` {
			continue
		}
		seen := false
		for _, existing := range conflicts[key] {
			if stableValue(existing) == stableValue(value) {
				seen = true
				break
			}
		}
		if !seen {
			conflicts[key] = append(conflicts[key], value)
		}
	}
}

func appendUniqueString(values []string, value string, comparisonFold bool) []string {
	if value == "" {
		return values
	}
	comparison := value
	if comparisonFold {
		comparison = normalizeComparison(value)
	}
	for _, existing := range values {
		existingComparison := existing
		if comparisonFold {
			existingComparison = normalizeComparison(existing)
		}
		if existingComparison == comparison {
			return values
		}
	}
	return append(values, value)
}

func containsRelation(relations []extract.RelationDraft, candidate extract.RelationDraft) bool {
	key := relationKey(candidate)
	for _, relation := range relations {
		if relationKey(relation) == key {
			return true
		}
	}
	return false
}

func relationKey(relation extract.RelationDraft) string {
	return fmt.Sprintf("%s|%s|%.9f|%s", normalizeComparison(relation.Predicate), normalizeComparison(relation.TargetReference), relation.Confidence, normalizeComparison(relation.Evidence))
}

func containsCitation(citations []extract.Citation, candidate extract.Citation) bool {
	key := citationKey(candidate)
	for _, citation := range citations {
		if citationKey(citation) == key {
			return true
		}
	}
	return false
}

func citationKey(citation extract.Citation) string {
	return fmt.Sprintf("%s|%s|%09d|%09d|%09d|%s", citation.SourceID, citation.URI, citation.Page, citation.StartLine, citation.EndLine, normalizeComparison(citation.Evidence))
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(value, "\r", "")), " ")
}

func normalizeMarkdown(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(strings.TrimSpace(value), "\n")
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], " \t")
	}
	return strings.Join(lines, "\n")
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case string:
		return normalizeWhitespace(typed)
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, normalizeWhitespace(item))
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, normalizeValue(item))
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[strings.TrimSpace(key)] = normalizeValue(item)
		}
		return result
	default:
		return value
	}
}

func stableValue(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%T:%v", value, value)
	}
	return string(data)
}

func draftSortKey(draft extract.ConceptDraft) string {
	data, _ := json.Marshal(draft)
	return normalizeComparison(draft.Type) + "|" + normalizeComparison(draft.Title) + "|" + draft.TemporaryID + "|" + string(data)
}

func conceptSortKey(concept CanonicalConcept) string {
	copyConcept := concept
	copyConcept.ID = ""
	data, _ := json.Marshal(copyConcept)
	digest := sha256.Sum256(data)
	return normalizeComparison(concept.Title) + "|" + hex.EncodeToString(digest[:])
}
