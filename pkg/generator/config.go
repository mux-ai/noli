// Package generator owns the strict repository-local noli.yaml configuration
// and (in later phases) deterministic knowledge generation. It imports
// pkg/okf; pkg/okf never imports this package.
package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"noli/pkg/okf"
)

const (
	// PrimaryConfigName is Noli's canonical project configuration filename.
	PrimaryConfigName = "noli.yaml"
	// LegacyConfigName is accepted for projects that have not migrated yet.
	LegacyConfigName = "okf.yaml"
	// MaxConfigBytes caps the project configuration file size.
	MaxConfigBytes = 1 << 20 // 1 MiB
)

// Sentinel errors for CLI error-code mapping.
var (
	// ErrNotFound marks a missing or unreadable configuration file.
	ErrNotFound = errors.New("configuration not found")
	// ErrUnsafePath marks a path that violates the frozen path rules.
	ErrUnsafePath = errors.New("unsafe path")
)

// Config is the strict noli.yaml model (docs/PROTOCOL.md section 9). Unknown
// fields are rejected.
type Config struct {
	Version       int                  `yaml:"version"`
	Project       ProjectConfig        `yaml:"project"`
	Knowledge     KnowledgeConfig      `yaml:"knowledge"`
	Sources       []SourceConfig       `yaml:"sources"`
	ConceptTypes  []ConceptTypeConfig  `yaml:"concept_types"`
	Relationships []RelationshipConfig `yaml:"relationships"`
	Retrieval     RetrievalConfig      `yaml:"retrieval"`
	Agent         AgentConfig          `yaml:"agent"`
	Security      SecurityConfig       `yaml:"security"`
	Generation    GenerationConfig     `yaml:"generation"`

	dir string // absolute directory containing noli.yaml (or a legacy config)
}

// ProjectConfig names the project and carries project-wide validation rules.
type ProjectConfig struct {
	Name              string   `yaml:"name"`
	Description       string   `yaml:"description"`
	RequiredMetadata  []string `yaml:"required_metadata"`
	RequireConfidence bool     `yaml:"require_confidence"`
	MinimumConfidence float64  `yaml:"minimum_confidence"`
	RequireCitations  bool     `yaml:"require_citations"`
}

// KnowledgeConfig locates the knowledge bundle relative to noli.yaml.
type KnowledgeConfig struct {
	Root string `yaml:"root"`
}

// SourceConfig is one generation input location.
type SourceConfig struct {
	Path string `yaml:"path"`
	Type string `yaml:"type"`
}

// ConceptTypeConfig is one allowed concept type.
type ConceptTypeConfig struct {
	Type             string   `yaml:"type"`
	Aliases          []string `yaml:"aliases"`
	Directory        string   `yaml:"directory"`
	RequiredFields   []string `yaml:"required_fields"`
	RequiredSections []string `yaml:"required_sections"`
	IdentityFields   []string `yaml:"identity_fields"`
}

// RelationshipConfig is one allowed typed relationship.
type RelationshipConfig struct {
	Predicate string `yaml:"predicate"`
	From      string `yaml:"from"`
	To        string `yaml:"to"`
}

// RetrievalConfig overrides the frozen retrieval defaults.
type RetrievalConfig struct {
	SearchLimit   int `yaml:"search_limit"`
	MaxHops       int `yaml:"max_hops"`
	MaxDocuments  int `yaml:"max_documents"`
	MaxCharacters int `yaml:"max_characters"`
}

// AgentConfig locates agent-facing assets.
type AgentConfig struct {
	QueriesFile string `yaml:"queries_file"`
}

// SecurityConfig adds bundle exclusions on top of the built-in rules.
type SecurityConfig struct {
	Exclude []string `yaml:"exclude"`
}

// GenerationConfig lists deterministic structured concept inputs: inline
// concepts and/or external concept files.
type GenerationConfig struct {
	ConceptFiles []string       `yaml:"concept_files"`
	Concepts     []ConceptInput `yaml:"concepts"`
}

// LoadConfig strictly parses and validates noli.yaml. All relative paths are
// resolved against the directory containing the file.
func LoadConfig(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("configuration path is empty: %w", ErrNotFound)
	}
	if strings.ContainsRune(path, '\x00') {
		return nil, fmt.Errorf("configuration path contains a NUL byte: %w", ErrUnsafePath)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve configuration path %q: %w", path, ErrNotFound)
	}
	absPath = preferPrimaryConfig(absPath)
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("configuration file %q does not exist: %w", path, ErrNotFound)
	}
	if info.Size() > MaxConfigBytes {
		return nil, fmt.Errorf("configuration file %q exceeds the %d byte limit", path, MaxConfigBytes)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read configuration %q: %w", path, ErrNotFound)
	}

	config := &Config{}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("parse configuration %q: %w", path, err)
	}
	config.dir = filepath.Dir(absPath)
	if err := config.validate(); err != nil {
		return nil, err
	}
	return config, nil
}

func preferPrimaryConfig(path string) string {
	if filepath.Base(path) != LegacyConfigName {
		return path
	}
	primary := filepath.Join(filepath.Dir(path), PrimaryConfigName)
	if info, err := os.Stat(primary); err == nil && !info.IsDir() {
		return primary
	}
	return path
}

// Dir returns the absolute directory containing noli.yaml.
func (c *Config) Dir() string { return c.dir }

func (c *Config) validate() error {
	if c.Version != 1 {
		return fmt.Errorf("configuration version must be 1, got %d", c.Version)
	}
	if strings.TrimSpace(c.Project.Name) == "" {
		return fmt.Errorf("project.name is required")
	}
	if c.Project.MinimumConfidence < 0 || c.Project.MinimumConfidence > 1 {
		return fmt.Errorf("project.minimum_confidence must be between 0 and 1")
	}
	if err := safeRelativePath("knowledge.root", c.Knowledge.Root); err != nil {
		return err
	}
	if len(c.ConceptTypes) == 0 {
		return fmt.Errorf("concept_types must list at least one type")
	}
	seenTypes := make(map[string]string)
	for i, conceptType := range c.ConceptTypes {
		location := fmt.Sprintf("concept_types[%d]", i)
		if strings.TrimSpace(conceptType.Type) == "" {
			return fmt.Errorf("%s.type is required", location)
		}
		if err := safeRelativePath(location+".directory", conceptType.Directory); err != nil {
			return err
		}
		names := append([]string{conceptType.Type}, conceptType.Aliases...)
		for _, name := range names {
			normalized := strings.ToLower(strings.TrimSpace(name))
			if normalized == "" {
				return fmt.Errorf("%s has an empty alias", location)
			}
			if previous, duplicate := seenTypes[normalized]; duplicate {
				return fmt.Errorf("%s name %q duplicates %q", location, name, previous)
			}
			seenTypes[normalized] = conceptType.Type
		}
	}
	for i, relationship := range c.Relationships {
		if strings.TrimSpace(relationship.Predicate) == "" {
			return fmt.Errorf("relationships[%d].predicate is required", i)
		}
	}
	if c.Retrieval.SearchLimit < 0 || c.Retrieval.MaxHops < 0 ||
		c.Retrieval.MaxDocuments < 0 || c.Retrieval.MaxCharacters < 0 {
		return fmt.Errorf("retrieval bounds must not be negative")
	}
	for i, source := range c.Sources {
		if err := safeRelativePath(fmt.Sprintf("sources[%d].path", i), source.Path); err != nil {
			return err
		}
	}
	if c.Agent.QueriesFile != "" {
		if err := safeRelativePath("agent.queries_file", c.Agent.QueriesFile); err != nil {
			return err
		}
	}
	for i, exclusion := range c.Security.Exclude {
		if err := safeExclusion(fmt.Sprintf("security.exclude[%d]", i), exclusion); err != nil {
			return err
		}
	}
	for i, conceptFile := range c.Generation.ConceptFiles {
		if err := safeRelativePath(fmt.Sprintf("generation.concept_files[%d]", i), conceptFile); err != nil {
			return err
		}
	}
	return nil
}

// KnowledgeRoot resolves the configured knowledge root. A symlinked root is
// accepted only when its resolved target stays inside the resolved project
// directory.
func (c *Config) KnowledgeRoot() (string, error) {
	root := filepath.Join(c.dir, filepath.FromSlash(c.Knowledge.Root))
	resolvedDir, err := filepath.EvalSymlinks(c.dir)
	if err != nil {
		return "", fmt.Errorf("resolve project directory %q: %w", c.dir, ErrNotFound)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		// The root may not exist yet (before first generation); return the
		// unresolved path and let the caller report a load failure.
		return root, nil
	}
	rel, err := filepath.Rel(resolvedDir, resolvedRoot)
	if err != nil || relEscapes(rel) {
		return "", fmt.Errorf("knowledge root %q escapes the project directory after symlink resolution: %w",
			c.Knowledge.Root, ErrUnsafePath)
	}
	return resolvedRoot, nil
}

// ValidationOptions maps the configuration onto SDK validation options.
func (c *Config) ValidationOptions() okf.ValidationOptions {
	rules := &okf.ProjectRules{
		RequiredMetadata:  c.Project.RequiredMetadata,
		RequireConfidence: c.Project.RequireConfidence,
		MinimumConfidence: c.Project.MinimumConfidence,
		RequireCitations:  c.Project.RequireCitations,
	}
	for _, conceptType := range c.ConceptTypes {
		rules.ConceptTypes = append(rules.ConceptTypes, okf.ConceptTypeRule{
			Type:             conceptType.Type,
			Aliases:          conceptType.Aliases,
			Directory:        conceptType.Directory,
			RequiredMetadata: conceptType.RequiredFields,
			RequiredSections: conceptType.RequiredSections,
			IdentityFields:   conceptType.IdentityFields,
		})
	}
	return okf.ValidationOptions{
		Project: rules,
		Parse:   okf.ParseOptions{Exclude: c.Security.Exclude},
	}
}

// safeRelativePath enforces the frozen path rules: non-empty, relative,
// slash-separated, no NUL, no backslash, no traversal, no sensitive
// components.
func safeRelativePath(location, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", location)
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s contains a NUL byte: %w", location, ErrUnsafePath)
	}
	if strings.Contains(value, "\\") {
		return fmt.Errorf("%s contains a backslash: %w", location, ErrUnsafePath)
	}
	if filepath.IsAbs(value) {
		return fmt.Errorf("%s must be relative, got %q: %w", location, value, ErrUnsafePath)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("%s escapes the project directory: %w", location, ErrUnsafePath)
	}
	if slices.ContainsFunc(strings.Split(clean, "/"), sensitiveComponent) {
		return fmt.Errorf("%s uses a sensitive path component %q: %w", location, value, ErrUnsafePath)
	}
	return nil
}

// sensitiveComponent rejects the frozen sensitive names (docs/PROTOCOL.md
// section 9). Unlike bundle discovery, config paths may use hidden
// directories such as .noli; only the named sensitive components are refused.
func sensitiveComponent(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case ".git", "secrets", "credentials", "node_modules", "vendor", "build":
		return true
	}
	if strings.HasPrefix(lower, ".env") {
		return true
	}
	if strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") || strings.Contains(lower, "id_rsa") {
		return true
	}
	return false
}

// safeExclusion validates a security.exclude entry (a component name or a
// relative path prefix).
func safeExclusion(location, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is empty", location)
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s contains a NUL byte: %w", location, ErrUnsafePath)
	}
	if strings.Contains(value, "\\") {
		return fmt.Errorf("%s contains a backslash: %w", location, ErrUnsafePath)
	}
	if filepath.IsAbs(value) || strings.Contains(value, "..") {
		return fmt.Errorf("%s must be a relative component or prefix: %w", location, ErrUnsafePath)
	}
	return nil
}

func relEscapes(rel string) bool {
	clean := filepath.Clean(rel)
	return clean == ".." || filepath.IsAbs(clean) ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator))
}
