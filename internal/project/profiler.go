package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"noli/internal/llm"
)

const ProfilerSystemPrompt = `You design an extraction profile for an Open Knowledge Format bundle.

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

type SourceExcerpt struct {
	SourceID string `json:"source_id"`
	Name     string `json:"name"`
	URI      string `json:"uri,omitempty"`
	Content  string `json:"content"`
}

type ProfileRequest struct {
	ProjectName        string          `json:"project_name"`
	ProjectDescription string          `json:"project_description,omitempty"`
	Goal               string          `json:"goal"`
	Hints              []string        `json:"hints,omitempty"`
	SourceExcerpts     []SourceExcerpt `json:"representative_source_excerpts,omitempty"`
	MaximumExcerpts    int             `json:"-"`
	MaximumExcerptSize int             `json:"-"`
}

func GenerateProfile(ctx context.Context, client llm.Client, request ProfileRequest) (ProjectProfile, error) {
	if client == nil {
		return ProjectProfile{}, fmt.Errorf("generate project profile: LLM client is nil")
	}
	request.ProjectName = strings.TrimSpace(request.ProjectName)
	request.ProjectDescription = strings.TrimSpace(request.ProjectDescription)
	request.Goal = strings.TrimSpace(request.Goal)
	request.Hints = normalizedList(request.Hints, false)
	if err := ValidateProjectName(request.ProjectName); err != nil {
		return ProjectProfile{}, fmt.Errorf("generate project profile: invalid project name: %w", err)
	}
	if request.Goal == "" {
		return ProjectProfile{}, fmt.Errorf("generate project profile: goal must not be empty")
	}
	request.SourceExcerpts = prepareExcerpts(request.SourceExcerpts, request.MaximumExcerpts, request.MaximumExcerptSize)

	promptPayload := struct {
		ProfileRequest
		OutputSchema json.RawMessage `json:"output_schema"`
	}{ProfileRequest: request, OutputSchema: json.RawMessage(JSONSchema)}
	prompt, err := json.MarshalIndent(promptPayload, "", "  ")
	if err != nil {
		return ProjectProfile{}, fmt.Errorf("generate project profile: encode prompt: %w", err)
	}

	var raw json.RawMessage
	if err := client.GenerateStructured(ctx, ProfilerSystemPrompt, string(prompt), &raw); err != nil {
		return ProjectProfile{}, fmt.Errorf("generate project profile with Ollama: %w", err)
	}
	profile, err := DecodeProfileJSON(raw)
	if err != nil {
		return ProjectProfile{}, fmt.Errorf("generate project profile: validate structured response: %w", err)
	}
	if profile.Project.Name != request.ProjectName {
		return ProjectProfile{}, fmt.Errorf("generate project profile: response project name %q does not match %q", profile.Project.Name, request.ProjectName)
	}
	return profile, nil
}

func GenerateAndSaveProfile(ctx context.Context, client llm.Client, request ProfileRequest, filename string) (ProjectProfile, error) {
	profile, err := GenerateProfile(ctx, client, request)
	if err != nil {
		return ProjectProfile{}, err
	}
	if err := SaveProfileJSON(filename, profile); err != nil {
		return ProjectProfile{}, fmt.Errorf("generate project profile: persist generated profile: %w", err)
	}
	return profile, nil
}

func LoadProfile(filename string) (ProjectProfile, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return ProjectProfile{}, fmt.Errorf("read project profile %s: %w", filename, err)
	}
	switch strings.ToLower(filepathExtension(filename)) {
	case ".json":
		profile, err := DecodeProfileJSON(data)
		if err != nil {
			return ProjectProfile{}, fmt.Errorf("load project profile %s: %w", filename, err)
		}
		return profile, nil
	case ".yaml", ".yml":
		profile, err := DecodeProfileYAML(data)
		if err != nil {
			return ProjectProfile{}, fmt.Errorf("load project profile %s: %w", filename, err)
		}
		return profile, nil
	default:
		return ProjectProfile{}, fmt.Errorf("load project profile %s: expected .json, .yaml, or .yml", filename)
	}
}

func LoadNamedProfile(profilesDirectory, name string) (ProjectProfile, error) {
	if err := ValidateProjectName(name); err != nil {
		return ProjectProfile{}, fmt.Errorf("load named profile %q: %w", name, err)
	}
	filename, err := SafeJoin(profilesDirectory, name+".yaml")
	if err != nil {
		return ProjectProfile{}, fmt.Errorf("load named profile %q: %w", name, err)
	}
	return LoadProfile(filename)
}

func DecodeProfileJSON(data []byte) (ProjectProfile, error) {
	if err := requireProfileJSONFields(data); err != nil {
		return ProjectProfile{}, err
	}
	var profile ProjectProfile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&profile); err != nil {
		return ProjectProfile{}, fmt.Errorf("decode profile JSON: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return ProjectProfile{}, fmt.Errorf("decode profile JSON: %w", err)
	}
	return validatedNormalizedProfile(profile)
}

func DecodeProfileYAML(data []byte) (ProjectProfile, error) {
	var profile ProjectProfile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&profile); err != nil {
		return ProjectProfile{}, fmt.Errorf("decode profile YAML: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return ProjectProfile{}, fmt.Errorf("decode profile YAML: multiple documents are not allowed")
		}
		return ProjectProfile{}, fmt.Errorf("decode profile YAML trailing content: %w", err)
	}
	return validatedNormalizedProfile(profile)
}

func SaveProfileJSON(filename string, profile ProjectProfile) error {
	profile, err := validatedNormalizedProfile(profile)
	if err != nil {
		return fmt.Errorf("save project profile %s: %w", filename, err)
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("encode project profile %s as JSON: %w", filename, err)
	}
	data = append(data, '\n')
	if err := AtomicWriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("save project profile %s: %w", filename, err)
	}
	return nil
}

func SaveProfileYAML(filename string, profile ProjectProfile) error {
	profile, err := validatedNormalizedProfile(profile)
	if err != nil {
		return fmt.Errorf("save project profile %s: %w", filename, err)
	}
	data, err := yaml.Marshal(profile)
	if err != nil {
		return fmt.Errorf("encode project profile %s as YAML: %w", filename, err)
	}
	if err := AtomicWriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("save project profile %s: %w", filename, err)
	}
	return nil
}

func validatedNormalizedProfile(profile ProjectProfile) (ProjectProfile, error) {
	if err := ValidateProfile(profile); err != nil {
		return ProjectProfile{}, err
	}
	profile = ApplyDefaults(NormalizeProfile(profile))
	if err := ValidateProfile(profile); err != nil {
		return ProjectProfile{}, err
	}
	return profile, nil
}

func requireProfileJSONFields(data []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("decode profile JSON: %w", err)
	}
	for _, field := range []string{"version", "project", "okf", "concept_types", "relationships", "extraction"} {
		if _, exists := root[field]; !exists {
			return fmt.Errorf("profile JSON is missing required field %q", field)
		}
	}
	if err := requireObjectFields(root["project"], "project", []string{"name"}); err != nil {
		return err
	}
	if err := requireObjectFields(root["extraction"], "extraction", []string{
		"minimum_confidence", "require_source_evidence", "allow_inferred_relationships", "maximum_concepts_per_source",
	}); err != nil {
		return err
	}
	var concepts []map[string]json.RawMessage
	if err := json.Unmarshal(root["concept_types"], &concepts); err != nil {
		return fmt.Errorf("profile JSON field %q must be an array: %w", "concept_types", err)
	}
	for i, concept := range concepts {
		for _, field := range []string{"type", "directory", "identity_fields", "sections"} {
			if _, exists := concept[field]; !exists {
				return fmt.Errorf("profile JSON concept_types[%d] is missing required field %q", i, field)
			}
		}
	}
	var relationships []map[string]json.RawMessage
	if err := json.Unmarshal(root["relationships"], &relationships); err != nil {
		return fmt.Errorf("profile JSON field %q must be an array: %w", "relationships", err)
	}
	for i, relationship := range relationships {
		for _, field := range []string{"source_type", "relation", "target_type"} {
			if _, exists := relationship[field]; !exists {
				return fmt.Errorf("profile JSON relationships[%d] is missing required field %q", i, field)
			}
		}
	}
	return nil
}

func requireObjectFields(raw json.RawMessage, name string, fields []string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return fmt.Errorf("profile JSON field %q must be an object: %w", name, err)
	}
	for _, field := range fields {
		if _, exists := object[field]; !exists {
			return fmt.Errorf("profile JSON object %q is missing required field %q", name, field)
		}
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("multiple JSON values are not allowed")
	}
	return err
}

func prepareExcerpts(excerpts []SourceExcerpt, maximum, maximumSize int) []SourceExcerpt {
	result := append([]SourceExcerpt(nil), excerpts...)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].SourceID == result[j].SourceID {
			return result[i].Name < result[j].Name
		}
		return result[i].SourceID < result[j].SourceID
	})
	if maximum <= 0 {
		maximum = DefaultMaximumSourceExcerpts
	}
	if len(result) > maximum {
		result = result[:maximum]
	}
	for i := range result {
		result[i].SourceID = strings.TrimSpace(result[i].SourceID)
		result[i].Name = strings.TrimSpace(result[i].Name)
		result[i].URI = strings.TrimSpace(result[i].URI)
		result[i].Content = strings.TrimSpace(result[i].Content)
		if maximumSize > 0 && len(result[i].Content) > maximumSize {
			result[i].Content = truncateUTF8(result[i].Content, maximumSize)
		}
	}
	return result
}

func truncateUTF8(value string, maximum int) string {
	if maximum <= 0 || len(value) <= maximum {
		return value
	}
	cut := maximum
	for cut > 0 && (value[cut]&0xc0) == 0x80 {
		cut--
	}
	return strings.TrimSpace(value[:cut])
}

func filepathExtension(filename string) string {
	lastSeparator := strings.LastIndexAny(filename, "/\\")
	lastDot := strings.LastIndex(filename, ".")
	if lastDot <= lastSeparator {
		return ""
	}
	return filename[lastDot:]
}
