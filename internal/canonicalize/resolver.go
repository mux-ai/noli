package canonicalize

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"noli/internal/project"
)

type Edge struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Predicate  string  `json:"predicate"`
	Confidence float64 `json:"confidence"`
	Evidence   string  `json:"evidence,omitempty"`
}

type UnresolvedRelation struct {
	From            string  `json:"from"`
	Predicate       string  `json:"predicate"`
	TargetReference string  `json:"target_reference"`
	Confidence      float64 `json:"confidence"`
	Evidence        string  `json:"evidence,omitempty"`
	Reason          string  `json:"reason"`
}

func ResolveRelationships(profile project.ProjectProfile, concepts []CanonicalConcept) ([]Edge, []UnresolvedRelation) {
	sorted := append([]CanonicalConcept(nil), concepts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	configs := conceptTypeConfigs(profile)
	edges := make([]Edge, 0)
	unresolved := make([]UnresolvedRelation, 0)
	for _, sourceConcept := range sorted {
		relations := append([]projectRelationship(nil), relationshipsFor(sourceConcept)...)
		sort.Slice(relations, func(i, j int) bool { return relations[i].key() < relations[j].key() })
		for _, relation := range relations {
			base := UnresolvedRelation{From: sourceConcept.ID, Predicate: relation.Predicate, TargetReference: relation.TargetReference, Confidence: relation.Confidence, Evidence: relation.Evidence}
			if relation.Confidence < profile.Extraction.MinimumConfidence {
				base.Reason = "relationship confidence is below the project minimum"
				unresolved = append(unresolved, base)
				continue
			}
			allowedTypes := relationshipTargetTypes(profile, sourceConcept.Type, relation.Predicate)
			matches := resolveCandidates(relation.TargetReference, sorted, configs, allowedTypes)
			if len(matches) == 0 {
				base.Reason = "no matching canonical concept"
				unresolved = append(unresolved, base)
				continue
			}
			if len(matches) > 1 {
				base.Reason = "target reference is ambiguous: " + strings.Join(matches, ", ")
				unresolved = append(unresolved, base)
				continue
			}
			edges = append(edges, Edge{From: sourceConcept.ID, To: matches[0], Predicate: relation.Predicate, Confidence: relation.Confidence, Evidence: relation.Evidence})
		}
	}
	edges = deduplicateEdges(edges)
	sort.Slice(edges, func(i, j int) bool { return edgeKey(edges[i]) < edgeKey(edges[j]) })
	sort.Slice(unresolved, func(i, j int) bool { return unresolvedKey(unresolved[i]) < unresolvedKey(unresolved[j]) })
	return edges, unresolved
}

type projectRelationship struct {
	Predicate       string
	TargetReference string
	Evidence        string
	Confidence      float64
}

func (r projectRelationship) key() string {
	return fmt.Sprintf("%s|%s|%.9f|%s", normalizeComparison(r.Predicate), normalizeComparison(r.TargetReference), r.Confidence, normalizeComparison(r.Evidence))
}

func relationshipsFor(concept CanonicalConcept) []projectRelationship {
	result := make([]projectRelationship, 0, len(concept.Relations))
	for _, relation := range concept.Relations {
		result = append(result, projectRelationship{Predicate: relation.Predicate, TargetReference: relation.TargetReference, Evidence: relation.Evidence, Confidence: relation.Confidence})
	}
	return result
}

func relationshipTargetTypes(profile project.ProjectProfile, sourceType, predicate string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, rule := range profile.Relationships {
		if normalizeComparison(rule.SourceType) == normalizeComparison(sourceType) && normalizeComparison(rule.Relation) == normalizeComparison(predicate) {
			result[normalizeComparison(rule.TargetType)] = struct{}{}
		}
	}
	return result
}

func resolveCandidates(reference string, concepts []CanonicalConcept, configs map[string]project.ConceptTypeConfig, allowedTypes map[string]struct{}) []string {
	candidates := filterByTypes(concepts, allowedTypes)
	refID := normalizeReferenceID(reference)
	exactIDs := make([]string, 0)
	for _, concept := range candidates {
		if normalizeReferenceID(concept.ID) == refID {
			exactIDs = append(exactIDs, concept.ID)
		}
	}
	if len(exactIDs) > 0 {
		return sortedUnique(exactIDs)
	}
	normalizedReference := normalizeComparison(reference)
	exactNames := make([]string, 0)
	for _, concept := range candidates {
		if normalizeComparison(concept.Title) == normalizedReference {
			exactNames = append(exactNames, concept.ID)
			continue
		}
		for _, alias := range concept.Aliases {
			if normalizeComparison(alias) == normalizedReference {
				exactNames = append(exactNames, concept.ID)
				break
			}
		}
	}
	if len(exactNames) > 0 {
		return sortedUnique(exactNames)
	}
	identityMatches := make([]string, 0)
	for _, concept := range candidates {
		config := configs[normalizeComparison(concept.Type)]
		for _, field := range config.IdentityFields {
			value, ok := canonicalField(concept, field)
			if ok && normalizeAnyComparison(value) == normalizedReference {
				identityMatches = append(identityMatches, concept.ID)
				break
			}
		}
	}
	if len(identityMatches) > 0 {
		return sortedUnique(identityMatches)
	}

	referenceTokens := tokenSet(reference)
	if len(referenceTokens) < 2 {
		return nil
	}
	bestScore := 0.0
	best := make([]string, 0)
	for _, concept := range candidates {
		texts := append([]string{concept.Title}, concept.Aliases...)
		conceptScore := 0.0
		for _, text := range texts {
			tokens := tokenSet(text)
			intersection := setIntersectionSize(referenceTokens, tokens)
			if intersection < 2 {
				continue
			}
			union := len(referenceTokens) + len(tokens) - intersection
			if union == 0 {
				continue
			}
			score := float64(intersection) / float64(union)
			if score > conceptScore {
				conceptScore = score
			}
		}
		if conceptScore < 0.6 {
			continue
		}
		if conceptScore > bestScore {
			bestScore = conceptScore
			best = []string{concept.ID}
		} else if conceptScore == bestScore {
			best = append(best, concept.ID)
		}
	}
	return sortedUnique(best)
}

func filterByTypes(concepts []CanonicalConcept, allowedTypes map[string]struct{}) []CanonicalConcept {
	if len(allowedTypes) == 0 {
		return concepts
	}
	result := make([]CanonicalConcept, 0)
	for _, concept := range concepts {
		if _, allowed := allowedTypes[normalizeComparison(concept.Type)]; allowed {
			result = append(result, concept)
		}
	}
	return result
}

func normalizeReferenceID(reference string) string {
	reference = strings.TrimSpace(reference)
	if fragment := strings.IndexByte(reference, '#'); fragment >= 0 {
		reference = reference[:fragment]
	}
	reference = strings.TrimPrefix(reference, "/")
	reference = strings.TrimSuffix(reference, ".md")
	return strings.ToLower(strings.Trim(reference, "/"))
}

func tokenSet(value string) map[string]struct{} {
	result := make(map[string]struct{})
	var token strings.Builder
	flush := func() {
		if token.Len() == 0 {
			return
		}
		result[strings.ToLower(token.String())] = struct{}{}
		token.Reset()
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			token.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return result
}

func setIntersectionSize(left, right map[string]struct{}) int {
	count := 0
	for token := range left {
		if _, exists := right[token]; exists {
			count++
		}
	}
	return count
}

func sortedUnique(values []string) []string {
	sort.Strings(values)
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func deduplicateEdges(edges []Edge) []Edge {
	seen := make(map[string]struct{})
	result := make([]Edge, 0, len(edges))
	for _, edge := range edges {
		key := edgeKey(edge)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, edge)
	}
	return result
}

func edgeKey(edge Edge) string {
	return fmt.Sprintf("%s|%s|%s|%.9f|%s", edge.From, normalizeComparison(edge.Predicate), edge.To, edge.Confidence, normalizeComparison(edge.Evidence))
}

func unresolvedKey(relation UnresolvedRelation) string {
	return fmt.Sprintf("%s|%s|%s|%.9f|%s", relation.From, normalizeComparison(relation.Predicate), normalizeComparison(relation.TargetReference), relation.Confidence, relation.Reason)
}
