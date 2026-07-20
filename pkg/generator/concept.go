package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// MaxConceptFileBytes caps one structured concept input file.
const MaxConceptFileBytes = 4 << 20 // 4 MiB

// ConceptInput is one structured concept (docs/PLANS.md section 5.5). No LLM
// writes final Markdown; this structure is the only generation input.
type ConceptInput struct {
	// ID is optional; when absent it is derived from the type directory and
	// the title slug.
	ID            string              `yaml:"id" json:"id"`
	Type          string              `yaml:"type" json:"type"`
	Title         string              `yaml:"title" json:"title"`
	Description   string              `yaml:"description" json:"description"`
	Resource      string              `yaml:"resource" json:"resource"`
	Tags          []string            `yaml:"tags" json:"tags"`
	Attributes    map[string]any      `yaml:"attributes" json:"attributes"`
	Sections      []SectionInput      `yaml:"sections" json:"sections"`
	Relationships []RelationshipInput `yaml:"relationships" json:"relationships"`
	Citations     []CitationInput     `yaml:"citations" json:"citations"`
	// Timestamp is emitted only when explicitly supplied; generation never
	// invents time values.
	Timestamp string `yaml:"timestamp" json:"timestamp"`
}

// SectionInput is one Markdown section in input order.
type SectionInput struct {
	Heading string `yaml:"heading" json:"heading"`
	Content string `yaml:"content" json:"content"`
}

// RelationshipInput points at another generated concept by canonical ID or
// exact title.
type RelationshipInput struct {
	Predicate string `yaml:"predicate" json:"predicate"`
	To        string `yaml:"to" json:"to"`
}

// CitationInput is one source citation.
type CitationInput struct {
	Source    string `yaml:"source" json:"source"`
	URI       string `yaml:"uri" json:"uri"`
	Page      int    `yaml:"page" json:"page"`
	StartLine int    `yaml:"start_line" json:"start_line"`
	EndLine   int    `yaml:"end_line" json:"end_line"`
	Evidence  string `yaml:"evidence" json:"evidence"`
}

// conceptFile is the strict schema of an external concept file.
type conceptFile struct {
	Concepts []ConceptInput `yaml:"concepts" json:"concepts"`
}

// loadConcepts gathers inline concepts and external concept files in
// deterministic order: inline first, then files in configuration order.
func loadConcepts(config *Config) ([]ConceptInput, error) {
	concepts := append([]ConceptInput{}, config.Generation.Concepts...)
	for _, relative := range config.Generation.ConceptFiles {
		path := filepath.Join(config.dir, filepath.FromSlash(relative))
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("concept file %q does not exist", relative)
		}
		if info.Size() > MaxConceptFileBytes {
			return nil, fmt.Errorf("concept file %q exceeds the %d byte limit", relative, MaxConceptFileBytes)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read concept file %q: %w", relative, err)
		}
		var file conceptFile
		decoder := yaml.NewDecoder(strings.NewReader(string(data)))
		decoder.KnownFields(true)
		if err := decoder.Decode(&file); err != nil {
			return nil, fmt.Errorf("parse concept file %q: %w", relative, err)
		}
		concepts = append(concepts, file.Concepts...)
	}
	return concepts, nil
}

// resolvedConcept is a validated concept with its canonical identity and
// resolved relationship targets.
type resolvedConcept struct {
	ID            string
	Directory     string
	Type          string
	Input         ConceptInput
	Relationships []resolvedRelationship
}

type resolvedRelationship struct {
	Predicate string
	TargetID  string
}

// resolveConcepts validates every concept against the configuration, creates
// canonical IDs, and resolves relationship targets by ID or exact title.
func resolveConcepts(config *Config, inputs []ConceptInput) ([]resolvedConcept, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no structured concept inputs are configured (generation.concepts or generation.concept_files)")
	}
	types := make(map[string]ConceptTypeConfig)
	for _, conceptType := range config.ConceptTypes {
		types[strings.ToLower(strings.TrimSpace(conceptType.Type))] = conceptType
		for _, alias := range conceptType.Aliases {
			types[strings.ToLower(strings.TrimSpace(alias))] = conceptType
		}
	}
	allowedPredicates := make(map[string]struct{})
	for _, relationship := range config.Relationships {
		allowedPredicates[strings.ToLower(strings.TrimSpace(relationship.Predicate))] = struct{}{}
	}

	resolved := make([]resolvedConcept, 0, len(inputs))
	byID := make(map[string]int)
	titleToID := make(map[string][]string)
	for i, input := range inputs {
		location := fmt.Sprintf("concept %d", i+1)
		if strings.TrimSpace(input.Title) != "" {
			location = fmt.Sprintf("concept %q", input.Title)
		}
		rule, known := types[strings.ToLower(strings.TrimSpace(input.Type))]
		if !known {
			return nil, fmt.Errorf("%s: type %q is not configured in noli.yaml", location, input.Type)
		}
		if strings.TrimSpace(input.Title) == "" {
			return nil, fmt.Errorf("%s: title is required", location)
		}
		for key := range input.Attributes {
			if isReservedAttribute(key) {
				return nil, fmt.Errorf("%s: attribute %q conflicts with a reserved metadata field", location, key)
			}
		}
		id, err := canonicalConceptID(rule, input)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", location, err)
		}
		if previous, duplicate := byID[id]; duplicate {
			return nil, fmt.Errorf("%s: canonical ID %q duplicates concept %d", location, id, previous+1)
		}
		byID[id] = i
		title := strings.ToLower(strings.Join(strings.Fields(input.Title), " "))
		titleToID[title] = append(titleToID[title], id)
		resolved = append(resolved, resolvedConcept{
			ID:        id,
			Directory: normalizedDirectory(rule.Directory),
			Type:      rule.Type,
			Input:     input,
		})
	}

	for i := range resolved {
		for _, relationship := range resolved[i].Input.Relationships {
			predicate := strings.ToLower(strings.TrimSpace(relationship.Predicate))
			if predicate == "" {
				return nil, fmt.Errorf("concept %q: relationship predicate is required", resolved[i].ID)
			}
			if len(allowedPredicates) > 0 {
				if _, allowed := allowedPredicates[predicate]; !allowed {
					return nil, fmt.Errorf("concept %q: predicate %q is not configured in noli.yaml relationships",
						resolved[i].ID, relationship.Predicate)
				}
			}
			targetID, err := resolveTarget(relationship.To, byID, titleToID)
			if err != nil {
				return nil, fmt.Errorf("concept %q: %w", resolved[i].ID, err)
			}
			resolved[i].Relationships = append(resolved[i].Relationships, resolvedRelationship{
				Predicate: predicate,
				TargetID:  targetID,
			})
		}
		sort.Slice(resolved[i].Relationships, func(a, b int) bool {
			left, right := resolved[i].Relationships[a], resolved[i].Relationships[b]
			if left.Predicate != right.Predicate {
				return left.Predicate < right.Predicate
			}
			return left.TargetID < right.TargetID
		})
	}
	sort.Slice(resolved, func(i, j int) bool { return resolved[i].ID < resolved[j].ID })
	return resolved, nil
}

func resolveTarget(target string, byID map[string]int, titleToID map[string][]string) (string, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return "", fmt.Errorf("relationship target is required")
	}
	if _, exists := byID[trimmed]; exists {
		return trimmed, nil
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(trimmed), " "))
	matches := titleToID[normalized]
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("relationship target %q cannot be resolved", target)
	default:
		return "", fmt.Errorf("relationship target %q is ambiguous between %s",
			target, strings.Join(matches, ", "))
	}
}

func isReservedAttribute(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "type", "title", "description", "resource", "tags", "timestamp":
		return true
	default:
		return false
	}
}
