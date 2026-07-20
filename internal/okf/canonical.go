package okf

import (
	"strings"

	"noli/internal/canonicalize"
	"noli/internal/project"
)

// ConceptsFromCanonical adapts the extraction pipeline's canonical model to
// the domain-independent renderer model without discarding review metadata.
func ConceptsFromCanonical(input []canonicalize.CanonicalConcept) []Concept {
	result := make([]Concept, 0, len(input))
	for _, canonical := range input {
		attributes := make(map[string]any, len(canonical.Attributes)+3)
		for key, value := range canonical.Attributes {
			attributes[key] = value
		}
		addCustomAttribute(attributes, "aliases", append([]string(nil), canonical.Aliases...))
		addCustomAttribute(attributes, "source_draft_ids", append([]string(nil), canonical.SourceDraftIDs...))
		if len(canonical.Conflicts) > 0 {
			addCustomAttribute(attributes, "conflicts", canonical.Conflicts)
		}
		sections := make([]Section, len(canonical.Sections))
		for i, section := range canonical.Sections {
			sections[i] = Section{Heading: section.Heading, Content: section.Content}
		}
		citations := make([]Citation, len(canonical.Citations))
		for i, citation := range canonical.Citations {
			citations[i] = Citation{
				SourceID: citation.SourceID, URI: citation.URI, Page: citation.Page,
				StartLine: citation.StartLine, EndLine: citation.EndLine, Evidence: citation.Evidence,
			}
		}
		result = append(result, Concept{
			ID: canonical.ID, Type: canonical.Type, Title: canonical.Title,
			Description: canonical.Description, Resource: canonical.Resource,
			Tags: append([]string(nil), canonical.Tags...), Confidence: canonical.Confidence,
			Attributes: attributes, Sections: sections, Citations: citations,
		})
	}
	return result
}

func RelationsFromCanonical(input []canonicalize.Edge) []Relation {
	result := make([]Relation, len(input))
	for i, edge := range input {
		result[i] = Relation{
			From: edge.From, To: edge.To, Predicate: edge.Predicate,
			Confidence: edge.Confidence, Evidence: edge.Evidence,
		}
	}
	return result
}

func RenderCanonicalBundle(root string, profile project.ProjectProfile, concepts []canonicalize.CanonicalConcept, edges []canonicalize.Edge, run RunInfo) error {
	return RenderBundle(root, profile, ConceptsFromCanonical(concepts), RelationsFromCanonical(edges), run)
}

func GenerateCanonicalBundle(destination string, profile project.ProjectProfile, concepts []canonicalize.CanonicalConcept, edges []canonicalize.Edge, run RunInfo, strictRelationships bool) ([]Problem, error) {
	return GenerateBundle(destination, profile, ConceptsFromCanonical(concepts), RelationsFromCanonical(edges), run, strictRelationships)
}

func addCustomAttribute(attributes map[string]any, key string, value any) {
	if !valuePresent(value) {
		return
	}
	for existing := range attributes {
		if strings.EqualFold(existing, key) {
			return
		}
	}
	attributes[key] = value
}
