package extract

import (
	"encoding/json"
	"fmt"

	"noli/internal/project"
)

const ProfileSystemPrompt = `You design an extraction profile for an Open Knowledge Format bundle.

Analyze the supplied project description and representative source documents.

Determine:
1. Important reusable concept types.
2. A safe directory for each concept type.
3. Identity fields for duplicate detection.
4. Useful Markdown sections.
5. Important relationships.
6. Domain-specific metadata.
7. Project validation rules.
8. Information requiring source evidence.

Do not generate final Markdown.
Do not create separate concept types when tags or metadata fields are sufficient.
Prefer reusable shared concepts over duplicated information.
Return only valid JSON matching the supplied schema.`

const ExtractionSystemPrompt = `You extract reusable knowledge concepts from source documents.

Follow the supplied project profile.

Rules:
- Use only information present in the supplied source content.
- Do not invent missing facts.
- Extract independent and reusable concepts.
- Separate shared concepts from entity-specific concepts.
- Include source evidence for factual claims.
- Express relationships using human-readable target references.
- Do not generate filenames.
- Do not generate filesystem paths.
- Do not generate YAML frontmatter.
- Do not generate final Markdown files.
- Use lower confidence when information is ambiguous.
- Return only valid JSON matching the supplied schema.`

const ConceptDraftJSONSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["concepts"],
  "properties": {
    "concepts": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["temporary_id", "type", "title", "description", "sections", "confidence"],
        "properties": {
          "temporary_id": {"type": "string", "minLength": 1},
          "type": {"type": "string", "minLength": 1},
          "title": {"type": "string", "minLength": 1},
          "description": {"type": "string", "minLength": 1},
          "resource": {"type": "string"},
          "tags": {"type": "array", "items": {"type": "string"}},
          "attributes": {"type": "object", "additionalProperties": true},
          "sections": {
            "type": "array",
            "items": {
              "type": "object", "additionalProperties": false,
              "required": ["heading", "content"],
              "properties": {"heading": {"type": "string", "minLength": 1}, "content": {"type": "string", "minLength": 1}}
            }
          },
          "relations": {
            "type": "array",
            "items": {
              "type": "object", "additionalProperties": false,
              "required": ["predicate", "target_reference", "confidence"],
              "properties": {
                "predicate": {"type": "string", "minLength": 1},
                "target_reference": {"type": "string", "minLength": 1},
                "evidence": {"type": "string"},
                "confidence": {"type": "number", "minimum": 0, "maximum": 1}
              }
            }
          },
          "citations": {
            "type": "array",
            "items": {
              "type": "object", "additionalProperties": false,
              "required": ["source_id"],
              "properties": {
                "source_id": {"type": "string", "minLength": 1}, "uri": {"type": "string"},
                "page": {"type": "integer", "minimum": 0}, "start_line": {"type": "integer", "minimum": 0},
                "end_line": {"type": "integer", "minimum": 0}, "evidence": {"type": "string"}
              }
            }
          },
          "confidence": {"type": "number", "minimum": 0, "maximum": 1}
        }
      }
    }
  }
}`

func BuildExtractionPrompt(profile project.ProjectProfile, excerpts []SourceExcerpt, maximumConcepts int) (string, error) {
	request := struct {
		Profile         project.ProjectProfile `json:"project_profile"`
		SourceExcerpts  []SourceExcerpt        `json:"source_excerpts"`
		MaximumConcepts int                    `json:"maximum_concepts"`
		OutputSchema    json.RawMessage        `json:"output_schema"`
	}{
		Profile:         profile,
		SourceExcerpts:  excerpts,
		MaximumConcepts: maximumConcepts,
		OutputSchema:    json.RawMessage(ConceptDraftJSONSchema),
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return "", fmt.Errorf("build extraction prompt: %w", err)
	}
	return string(data), nil
}

func BuildProfilePrompt(input any) (string, error) {
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", fmt.Errorf("build profile prompt: %w", err)
	}
	return string(data), nil
}
