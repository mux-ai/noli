package project

// ProjectProfile describes how domain-independent source material becomes an
// OKF bundle. Attribute names are intentionally not modeled here: extracted
// concepts retain arbitrary metadata.
type ProjectProfile struct {
	Version       int                    `json:"version" yaml:"version"`
	Project       ProjectInfo            `json:"project" yaml:"project"`
	OKF           OKFSettings            `json:"okf" yaml:"okf"`
	ConceptTypes  []ConceptTypeConfig    `json:"concept_types" yaml:"concept_types"`
	Relationships []RelationshipRule     `json:"relationships" yaml:"relationships"`
	Extraction    ExtractionSettings     `json:"extraction" yaml:"extraction"`
	Validation    ProjectValidationRules `json:"validation,omitempty" yaml:"validation,omitempty"`
}

type ProjectInfo struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Goal        string   `json:"goal,omitempty" yaml:"goal,omitempty"`
	Hints       []string `json:"hints,omitempty" yaml:"hints,omitempty"`
}

type OKFSettings struct {
	Title                     string   `json:"title,omitempty" yaml:"title,omitempty"`
	Description               string   `json:"description,omitempty" yaml:"description,omitempty"`
	LinkStyle                 string   `json:"link_style,omitempty" yaml:"link_style,omitempty"`
	IncludeEmptySections      bool     `json:"include_empty_sections,omitempty" yaml:"include_empty_sections,omitempty"`
	AllowDuplicateDirectories bool     `json:"allow_duplicate_directories,omitempty" yaml:"allow_duplicate_directories,omitempty"`
	SafetyInstructions        []string `json:"safety_instructions,omitempty" yaml:"safety_instructions,omitempty"`
}

type ConceptTypeConfig struct {
	Type             string   `json:"type" yaml:"type"`
	Directory        string   `json:"directory" yaml:"directory"`
	IdentityFields   []string `json:"identity_fields" yaml:"identity_fields"`
	Sections         []string `json:"sections" yaml:"sections"`
	RequiredFields   []string `json:"required_fields,omitempty" yaml:"required_fields,omitempty"`
	RequiredSections []string `json:"required_sections,omitempty" yaml:"required_sections,omitempty"`
	Aliases          []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`
}

type RelationshipRule struct {
	SourceType string `json:"source_type" yaml:"source_type"`
	Relation   string `json:"relation" yaml:"relation"`
	TargetType string `json:"target_type" yaml:"target_type"`
}

type ExtractionSettings struct {
	MinimumConfidence        float64 `json:"minimum_confidence" yaml:"minimum_confidence"`
	RequireSourceEvidence    bool    `json:"require_source_evidence" yaml:"require_source_evidence"`
	AllowInferredRelations   bool    `json:"allow_inferred_relationships" yaml:"allow_inferred_relationships"`
	MaximumConceptsPerSource int     `json:"maximum_concepts_per_source" yaml:"maximum_concepts_per_source"`
	MaximumChunkCharacters   int     `json:"maximum_chunk_characters,omitempty" yaml:"maximum_chunk_characters,omitempty"`
	MaximumSourceExcerpts    int     `json:"maximum_source_excerpts,omitempty" yaml:"maximum_source_excerpts,omitempty"`
	RequestTimeoutText       string  `json:"request_timeout,omitempty" yaml:"request_timeout,omitempty"`
}

// ProjectValidationRules holds optional project-level restrictions. Metadata
// remains open-ended; these rules only add checks requested by a profile.
type ProjectValidationRules struct {
	RequireCitations     bool     `json:"require_citations,omitempty" yaml:"require_citations,omitempty"`
	RejectEmptyDocuments bool     `json:"reject_empty_documents,omitempty" yaml:"reject_empty_documents,omitempty"`
	RequiredMetadata     []string `json:"required_metadata,omitempty" yaml:"required_metadata,omitempty"`
}

// JSONSchema is embedded in the automatic profiler prompt. It deliberately
// keeps profile fields closed so malformed model output is rejected by the
// structured response layer rather than silently ignored.
const JSONSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["version", "project", "okf", "concept_types", "relationships", "extraction"],
  "properties": {
    "version": {"type": "integer", "minimum": 1},
    "project": {
      "type": "object",
      "additionalProperties": false,
      "required": ["name"],
      "properties": {
        "name": {"type": "string", "minLength": 1},
        "description": {"type": "string"},
        "goal": {"type": "string"},
        "hints": {"type": "array", "items": {"type": "string"}}
      }
    },
    "okf": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "title": {"type": "string"},
        "description": {"type": "string"},
        "link_style": {"enum": ["root-relative", "relative", ""]},
        "include_empty_sections": {"type": "boolean"},
        "allow_duplicate_directories": {"type": "boolean"},
        "safety_instructions": {"type": "array", "items": {"type": "string"}}
      }
    },
    "concept_types": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["type", "directory", "identity_fields", "sections"],
        "properties": {
          "type": {"type": "string", "minLength": 1},
          "directory": {"type": "string", "minLength": 1},
          "identity_fields": {"type": "array", "minItems": 1, "items": {"type": "string", "minLength": 1}},
          "sections": {"type": "array", "items": {"type": "string", "minLength": 1}},
          "required_fields": {"type": "array", "items": {"type": "string", "minLength": 1}},
          "required_sections": {"type": "array", "items": {"type": "string", "minLength": 1}},
          "aliases": {"type": "array", "items": {"type": "string", "minLength": 1}}
        }
      }
    },
    "relationships": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["source_type", "relation", "target_type"],
        "properties": {
          "source_type": {"type": "string", "minLength": 1},
          "relation": {"type": "string", "minLength": 1},
          "target_type": {"type": "string", "minLength": 1}
        }
      }
    },
    "extraction": {
      "type": "object",
      "additionalProperties": false,
      "required": ["minimum_confidence", "require_source_evidence", "allow_inferred_relationships", "maximum_concepts_per_source"],
      "properties": {
        "minimum_confidence": {"type": "number", "minimum": 0, "maximum": 1},
        "require_source_evidence": {"type": "boolean"},
        "allow_inferred_relationships": {"type": "boolean"},
        "maximum_concepts_per_source": {"type": "integer", "minimum": 1},
        "maximum_chunk_characters": {"type": "integer", "minimum": 1},
        "maximum_source_excerpts": {"type": "integer", "minimum": 1},
        "request_timeout": {"type": "string"}
      }
    },
    "validation": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "require_citations": {"type": "boolean"},
        "reject_empty_documents": {"type": "boolean"},
        "required_metadata": {"type": "array", "items": {"type": "string", "minLength": 1}}
      }
    }
  }
}`
