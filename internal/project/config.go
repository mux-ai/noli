package project

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	CurrentVersion                = 1
	DefaultMaximumChunkCharacters = 12_000
	DefaultMaximumSourceExcerpts  = 8
	DefaultMaximumConcepts        = 50
	DefaultRequestTimeout         = 2 * time.Minute
)

// Config is the small, human-editable project.yaml file. Paths are fixed by
// the workspace layout and are therefore not accepted from untrusted config.
type Config struct {
	Version     int      `json:"version" yaml:"version"`
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Goal        string   `json:"goal,omitempty" yaml:"goal,omitempty"`
	Profile     string   `json:"profile,omitempty" yaml:"profile,omitempty"`
	Hints       []string `json:"hints,omitempty" yaml:"hints,omitempty"`
}

func DefaultConfig(name string) Config {
	return Config{
		Version: CurrentVersion,
		Name:    strings.TrimSpace(name),
		Profile: "auto",
	}
}

func (c Config) Validate() error {
	var problems []ValidationProblem
	if c.Version < 1 {
		problems = append(problems, ValidationProblem{Field: "version", Message: "must be at least 1"})
	}
	if err := ValidateProjectName(c.Name); err != nil {
		problems = append(problems, ValidationProblem{Field: "name", Message: err.Error()})
	}
	for i, hint := range c.Hints {
		if strings.TrimSpace(hint) == "" {
			problems = append(problems, ValidationProblem{Field: fmt.Sprintf("hints[%d]", i), Message: "must not be empty"})
		}
	}
	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

func NormalizeProfile(profile ProjectProfile) ProjectProfile {
	profile.Project.Name = strings.TrimSpace(profile.Project.Name)
	profile.Project.Description = strings.TrimSpace(profile.Project.Description)
	profile.Project.Goal = strings.TrimSpace(profile.Project.Goal)
	profile.Project.Hints = normalizedList(profile.Project.Hints, false)
	profile.OKF.Title = strings.TrimSpace(profile.OKF.Title)
	profile.OKF.Description = strings.TrimSpace(profile.OKF.Description)
	profile.OKF.LinkStyle = strings.ToLower(strings.TrimSpace(profile.OKF.LinkStyle))
	profile.OKF.SafetyInstructions = normalizedList(profile.OKF.SafetyInstructions, false)
	profile.Extraction.RequestTimeoutText = strings.TrimSpace(profile.Extraction.RequestTimeoutText)
	profile.Validation.RequiredMetadata = normalizedList(profile.Validation.RequiredMetadata, true)

	canonicalTypes := make(map[string]string, len(profile.ConceptTypes))
	for i := range profile.ConceptTypes {
		conceptType := &profile.ConceptTypes[i]
		conceptType.Type = strings.TrimSpace(conceptType.Type)
		if directory, err := ValidateRelativeDirectory(conceptType.Directory); err == nil {
			conceptType.Directory = directory
		} else {
			conceptType.Directory = strings.TrimSpace(conceptType.Directory)
		}
		conceptType.IdentityFields = normalizedList(conceptType.IdentityFields, true)
		conceptType.Sections = normalizedList(conceptType.Sections, false)
		conceptType.RequiredFields = normalizedList(conceptType.RequiredFields, true)
		conceptType.RequiredSections = normalizedList(conceptType.RequiredSections, false)
		conceptType.Aliases = normalizedList(conceptType.Aliases, false)
		canonicalTypes[strings.ToLower(conceptType.Type)] = conceptType.Type
	}
	sort.SliceStable(profile.ConceptTypes, func(i, j int) bool {
		left := strings.ToLower(profile.ConceptTypes[i].Type)
		right := strings.ToLower(profile.ConceptTypes[j].Type)
		if left == right {
			return profile.ConceptTypes[i].Directory < profile.ConceptTypes[j].Directory
		}
		return left < right
	})

	for i := range profile.Relationships {
		relationship := &profile.Relationships[i]
		relationship.SourceType = canonicalTypeName(canonicalTypes, relationship.SourceType)
		relationship.Relation = strings.TrimSpace(relationship.Relation)
		relationship.TargetType = canonicalTypeName(canonicalTypes, relationship.TargetType)
	}
	sort.SliceStable(profile.Relationships, func(i, j int) bool {
		left := profile.Relationships[i]
		right := profile.Relationships[j]
		leftKey := strings.ToLower(left.SourceType) + "\x00" + strings.ToLower(left.Relation) + "\x00" + strings.ToLower(left.TargetType)
		rightKey := strings.ToLower(right.SourceType) + "\x00" + strings.ToLower(right.Relation) + "\x00" + strings.ToLower(right.TargetType)
		return leftKey < rightKey
	})
	return profile
}

// ApplyDefaults supplies optional operational limits. It intentionally does
// not supply fields required by profile validation, so incomplete LLM output
// is never made to look valid.
func ApplyDefaults(profile ProjectProfile) ProjectProfile {
	if profile.OKF.LinkStyle == "" {
		profile.OKF.LinkStyle = "root-relative"
	}
	if profile.Extraction.MaximumChunkCharacters == 0 {
		profile.Extraction.MaximumChunkCharacters = DefaultMaximumChunkCharacters
	}
	if profile.Extraction.MaximumSourceExcerpts == 0 {
		profile.Extraction.MaximumSourceExcerpts = DefaultMaximumSourceExcerpts
	}
	if profile.Extraction.RequestTimeoutText == "" {
		profile.Extraction.RequestTimeoutText = DefaultRequestTimeout.String()
	}
	return profile
}

func (p ProjectProfile) ConceptType(name string) (ConceptTypeConfig, bool) {
	folded := strings.ToLower(strings.TrimSpace(name))
	for _, conceptType := range p.ConceptTypes {
		if strings.ToLower(conceptType.Type) == folded {
			return conceptType, true
		}
		for _, alias := range conceptType.Aliases {
			if strings.ToLower(alias) == folded {
				return conceptType, true
			}
		}
	}
	return ConceptTypeConfig{}, false
}

func (e ExtractionSettings) Timeout() (time.Duration, error) {
	if strings.TrimSpace(e.RequestTimeoutText) == "" {
		return DefaultRequestTimeout, nil
	}
	duration, err := time.ParseDuration(e.RequestTimeoutText)
	if err != nil {
		return 0, fmt.Errorf("parse extraction request timeout %q: %w", e.RequestTimeoutText, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("extraction request timeout must be positive")
	}
	return duration, nil
}

func canonicalTypeName(types map[string]string, value string) string {
	trimmed := strings.TrimSpace(value)
	if canonical, exists := types[strings.ToLower(trimmed)]; exists {
		return canonical
	}
	return trimmed
}

func normalizedList(values []string, lower bool) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		key := strings.ToLower(value)
		if value == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}
