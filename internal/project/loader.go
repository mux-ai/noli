package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ConfigFilename              = "project.yaml"
	InputDirectory              = "input"
	StagingDirectory            = "staging"
	KnowledgeDirectory          = "knowledge"
	NormalizedDirectory         = "normalized"
	GeneratedProfileFilename    = "generated-profile.json"
	ConceptDraftsFilename       = "concept-drafts.json"
	CanonicalConceptsFilename   = "canonical-concepts.json"
	UnresolvedRelationsFilename = "unresolved-relations.json"
)

type Workspace struct {
	Root                    string
	ConfigPath              string
	InputDir                string
	StagingDir              string
	NormalizedDir           string
	KnowledgeDir            string
	GeneratedProfilePath    string
	ConceptDraftsPath       string
	CanonicalConceptsPath   string
	UnresolvedRelationsPath string
	Config                  Config
}

func ValidateProjectName(name string) error {
	if name == "" || strings.TrimSpace(name) != name {
		return fmt.Errorf("must be non-empty without surrounding whitespace")
	}
	if strings.ContainsRune(name, '\x00') || name == "." || name == ".." {
		return fmt.Errorf("contains an unsafe path component")
	}
	if filepath.IsAbs(name) || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("must be a single relative path component")
	}
	return nil
}

// ResolveWorkspace resolves a project name below workspaceRoot without doing
// filesystem writes. Use LoadWorkspace when projectDir is already known.
func ResolveWorkspace(workspaceRoot, projectName string) (Workspace, error) {
	if err := ValidateProjectName(projectName); err != nil {
		return Workspace{}, fmt.Errorf("resolve workspace project %q: %w", projectName, err)
	}
	root, err := SafeJoin(workspaceRoot, projectName)
	if err != nil {
		return Workspace{}, fmt.Errorf("resolve workspace project %q: %w", projectName, err)
	}
	return workspacePaths(root), nil
}

func InitWorkspace(workspaceRoot, projectName string) (Workspace, error) {
	workspace, err := ResolveWorkspace(workspaceRoot, projectName)
	if err != nil {
		return Workspace{}, err
	}
	if _, err := os.Stat(workspace.ConfigPath); err == nil {
		return Workspace{}, fmt.Errorf("initialize project %q: %s already exists", projectName, workspace.ConfigPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Workspace{}, fmt.Errorf("initialize project %q: inspect %s: %w", projectName, workspace.ConfigPath, err)
	}
	for _, directory := range []string{workspace.InputDir, workspace.NormalizedDir, workspace.KnowledgeDir} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return Workspace{}, fmt.Errorf("initialize project %q: create directory %s: %w", projectName, directory, err)
		}
	}
	workspace.Config = DefaultConfig(projectName)
	if err := SaveConfig(workspace.ConfigPath, workspace.Config); err != nil {
		return Workspace{}, fmt.Errorf("initialize project %q: %w", projectName, err)
	}
	return workspace, nil
}

func OpenWorkspace(workspaceRoot, projectName string) (Workspace, error) {
	workspace, err := ResolveWorkspace(workspaceRoot, projectName)
	if err != nil {
		return Workspace{}, err
	}
	return loadWorkspacePaths(workspace)
}

// LoadWorkspace loads a project from an explicit directory. Fixed descendants
// are used for input, staging, and knowledge so project.yaml cannot redirect
// writes outside the project.
func LoadWorkspace(projectDir string) (Workspace, error) {
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return Workspace{}, fmt.Errorf("load workspace %s: resolve absolute path: %w", projectDir, err)
	}
	return loadWorkspacePaths(workspacePaths(filepath.Clean(abs)))
}

func loadWorkspacePaths(workspace Workspace) (Workspace, error) {
	info, err := os.Stat(workspace.Root)
	if err != nil {
		return Workspace{}, fmt.Errorf("load workspace %s: %w", workspace.Root, err)
	}
	if !info.IsDir() {
		return Workspace{}, fmt.Errorf("load workspace %s: not a directory", workspace.Root)
	}
	config, err := LoadConfig(workspace.ConfigPath)
	if err != nil {
		return Workspace{}, fmt.Errorf("load workspace %s: %w", workspace.Root, err)
	}
	workspace.Config = config
	return workspace, nil
}

func workspacePaths(root string) Workspace {
	staging := filepath.Join(root, StagingDirectory)
	return Workspace{
		Root:                    root,
		ConfigPath:              filepath.Join(root, ConfigFilename),
		InputDir:                filepath.Join(root, InputDirectory),
		StagingDir:              staging,
		NormalizedDir:           filepath.Join(staging, NormalizedDirectory),
		KnowledgeDir:            filepath.Join(root, KnowledgeDirectory),
		GeneratedProfilePath:    filepath.Join(staging, GeneratedProfileFilename),
		ConceptDraftsPath:       filepath.Join(staging, ConceptDraftsFilename),
		CanonicalConceptsPath:   filepath.Join(staging, CanonicalConceptsFilename),
		UnresolvedRelationsPath: filepath.Join(staging, UnresolvedRelationsFilename),
	}
}

func LoadConfig(filename string) (Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, fmt.Errorf("read project config %s: %w", filename, err)
	}
	var config Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode project config %s: %w", filename, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Config{}, fmt.Errorf("decode project config %s: multiple YAML documents are not allowed", filename)
		}
		return Config{}, fmt.Errorf("decode project config %s trailing content: %w", filename, err)
	}
	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate project config %s: %w", filename, err)
	}
	return config, nil
}

func SaveConfig(filename string, config Config) error {
	config.Name = strings.TrimSpace(config.Name)
	config.Description = strings.TrimSpace(config.Description)
	config.Goal = strings.TrimSpace(config.Goal)
	config.Profile = strings.TrimSpace(config.Profile)
	config.Hints = normalizedList(config.Hints, false)
	if err := config.Validate(); err != nil {
		return fmt.Errorf("save project config %s: %w", filename, err)
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode project config %s: %w", filename, err)
	}
	if err := AtomicWriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("save project config %s: %w", filename, err)
	}
	return nil
}

func SafeJoin(root string, elements ...string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root %s: %w", root, err)
	}
	for _, element := range elements {
		if strings.ContainsRune(element, '\x00') || filepath.IsAbs(element) {
			return "", fmt.Errorf("unsafe absolute or NUL-containing path %q", element)
		}
	}
	target := filepath.Join(append([]string{rootAbs}, elements...)...)
	relative, err := filepath.Rel(rootAbs, target)
	if err != nil {
		return "", fmt.Errorf("check path containment for %s: %w", target, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path %s escapes root %s", target, rootAbs)
	}
	return filepath.Clean(target), nil
}

func AtomicWriteFile(filename string, data []byte, mode os.FileMode) (returnErr error) {
	directory := filepath.Dir(filename)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create parent directory %s: %w", directory, err)
	}
	temporary, err := os.CreateTemp(directory, ".noli-write-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", filename, err)
	}
	temporaryName := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			if closeErr := temporary.Close(); closeErr != nil && returnErr == nil {
				returnErr = fmt.Errorf("close temporary file for %s: %w", filename, closeErr)
			}
		}
		if removeErr := os.Remove(temporaryName); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && returnErr == nil {
			returnErr = fmt.Errorf("clean temporary file for %s: %w", filename, removeErr)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return fmt.Errorf("set temporary file permissions for %s: %w", filename, err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write temporary file for %s: %w", filename, err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary file for %s: %w", filename, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file for %s: %w", filename, err)
	}
	closed = true
	if err := os.Rename(temporaryName, filename); err != nil {
		return fmt.Errorf("replace %s atomically: %w", filename, err)
	}
	return nil
}
