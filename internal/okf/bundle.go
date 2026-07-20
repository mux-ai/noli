package okf

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"noli/internal/project"
)

const GeneratorVersion = "0.1.0"

// RenderBundle renders a complete bundle into root. Callers replacing an
// active bundle should prefer GenerateBundle, which validates a temporary
// directory before swapping it into place.
func RenderBundle(root string, profile project.ProjectProfile, concepts []Concept, relations []Relation, run RunInfo) error {
	if err := project.ValidateProfile(profile); err != nil {
		return fmt.Errorf("render bundle: validate profile: %w", err)
	}
	if run.GeneratedAt.IsZero() {
		return fmt.Errorf("render bundle: generation timestamp is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("render bundle: create root %q: %w", root, err)
	}

	typeDirectories := make(map[string]string, len(profile.ConceptTypes))
	typeSections := make(map[string][]string, len(profile.ConceptTypes))
	directories := make(map[string]*DirectoryIndex, len(profile.ConceptTypes))
	directoryTypes := make(map[string][]string, len(profile.ConceptTypes))
	for _, conceptType := range profile.ConceptTypes {
		key := strings.ToLower(strings.TrimSpace(conceptType.Type))
		typeDirectories[key] = conceptType.Directory
		typeSections[conceptType.Type] = append([]string(nil), conceptType.Sections...)
		directoryTypes[conceptType.Directory] = append(directoryTypes[conceptType.Directory], conceptType.Type)
		if _, exists := directories[conceptType.Directory]; !exists {
			directories[conceptType.Directory] = &DirectoryIndex{Directory: conceptType.Directory}
		}
	}
	for directory, names := range directoryTypes {
		sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
		directories[directory].Type = strings.Join(names, " / ")
	}

	items := append([]Concept(nil), concepts...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	conceptByID := make(map[string]Concept, len(items))
	titles := make(map[string]string, len(items))
	for _, concept := range items {
		if err := ValidateConceptID(concept.ID); err != nil {
			return fmt.Errorf("render bundle: %w", err)
		}
		if _, exists := conceptByID[concept.ID]; exists {
			return fmt.Errorf("render bundle: duplicate concept ID %q", concept.ID)
		}
		directory, exists := typeDirectories[strings.ToLower(strings.TrimSpace(concept.Type))]
		if !exists {
			return fmt.Errorf("render bundle: concept %q has unconfigured type %q", concept.ID, concept.Type)
		}
		if path.Dir(concept.ID) != directory {
			return fmt.Errorf("render bundle: concept %q is outside configured directory %q", concept.ID, directory)
		}
		conceptByID[concept.ID] = concept
		titles[concept.ID] = concept.Title
		directories[directory].Entries = append(directories[directory].Entries, IndexEntry{
			ID: concept.ID, Type: concept.Type, Title: concept.Title, Description: concept.Description,
		})
	}
	for _, relation := range relations {
		if _, exists := conceptByID[relation.From]; !exists {
			return fmt.Errorf("render bundle: relation source %q is not a canonical concept", relation.From)
		}
		if _, exists := conceptByID[relation.To]; !exists {
			return fmt.Errorf("render bundle: relation target %q is not a canonical concept", relation.To)
		}
	}

	options := RenderOptions{
		Timestamp:            formatTimestamp(run.GeneratedAt),
		LinkStyle:            profile.OKF.LinkStyle,
		IncludeEmptySections: profile.OKF.IncludeEmptySections,
		SectionOrder:         typeSections,
	}
	for _, concept := range items {
		content, err := RenderConcept(concept, relations, titles, options)
		if err != nil {
			return fmt.Errorf("render bundle concept %q: %w", concept.ID, err)
		}
		outputPath, err := conceptOutputPath(root, concept.ID)
		if err != nil {
			return fmt.Errorf("render bundle concept %q path: %w", concept.ID, err)
		}
		if err := WriteFileAtomic(outputPath, content, 0o644); err != nil {
			return fmt.Errorf("render bundle concept %q: %w", concept.ID, err)
		}
	}

	directoryList := make([]DirectoryIndex, 0, len(directories))
	for _, directory := range directories {
		directoryList = append(directoryList, *directory)
	}
	sort.Slice(directoryList, func(i, j int) bool { return directoryList[i].Directory < directoryList[j].Directory })
	for _, directory := range directoryList {
		content, err := RenderDirectoryIndex(directory, options)
		if err != nil {
			return fmt.Errorf("render bundle directory index %q: %w", directory.Directory, err)
		}
		outputPath := filepath.Join(root, filepath.FromSlash(directory.Directory), "index.md")
		if err := WriteFileAtomic(outputPath, content, 0o644); err != nil {
			return fmt.Errorf("render bundle directory index %q: %w", directory.Directory, err)
		}
	}

	title := strings.TrimSpace(profile.OKF.Title)
	if title == "" {
		title = strings.TrimSpace(profile.Project.Name)
	}
	rootIndex, err := RenderRootIndex(title, profile.OKF.Description, directoryList, options)
	if err != nil {
		return fmt.Errorf("render bundle root index: %w", err)
	}
	if err := WriteFileAtomic(filepath.Join(root, "index.md"), rootIndex, 0o644); err != nil {
		return fmt.Errorf("render bundle root index: %w", err)
	}
	if run.Profile == "" {
		run.Profile = profile.Project.Name
	}
	if run.GeneratorVersion == "" {
		run.GeneratorVersion = GeneratorVersion
	}
	if run.CanonicalConcepts == 0 && len(items) > 0 {
		run.CanonicalConcepts = len(items)
	}
	logContent, err := RenderLog(run)
	if err != nil {
		return fmt.Errorf("render bundle log: %w", err)
	}
	if err := WriteFileAtomic(filepath.Join(root, "log.md"), logContent, 0o644); err != nil {
		return fmt.Errorf("render bundle log: %w", err)
	}
	return nil
}

// GenerateBundle renders and validates a temporary bundle, then replaces the
// destination. A failed render or validation leaves the active bundle intact.
func GenerateBundle(destination string, profile project.ProjectProfile, concepts []Concept, relations []Relation, run RunInfo, strictRelationships bool) ([]Problem, error) {
	absDestination, err := filepath.Abs(destination)
	if err != nil {
		return nil, fmt.Errorf("generate bundle: resolve destination %q: %w", destination, err)
	}
	if filepath.Dir(absDestination) == absDestination {
		return nil, fmt.Errorf("generate bundle: refusing broad destination %q", destination)
	}
	parent := filepath.Dir(absDestination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, fmt.Errorf("generate bundle: create parent %q: %w", parent, err)
	}
	temporary, err := os.MkdirTemp(parent, ".noli-bundle-*")
	if err != nil {
		return nil, fmt.Errorf("generate bundle: create temporary directory: %w", err)
	}
	temporaryExists := true
	defer func() {
		if temporaryExists {
			_ = os.RemoveAll(temporary)
		}
	}()
	if err := RenderBundle(temporary, profile, concepts, relations, run); err != nil {
		return nil, err
	}
	problems := ValidateBundle(temporary, ProjectMode, ValidationOptions{
		Profile:                 &profile,
		UnresolvedRelations:     run.UnresolvedRelations,
		StrictRelationships:     strictRelationships,
		RequireBundleLog:        true,
		RequireDirectoryIndexes: true,
	})
	if validationErr := ProblemsError(problems); validationErr != nil {
		return problems, fmt.Errorf("generate bundle: validate temporary bundle: %w", validationErr)
	}

	backup := ""
	if info, statErr := os.Stat(absDestination); statErr == nil {
		if !info.IsDir() {
			return problems, fmt.Errorf("generate bundle: destination %q is not a directory", destination)
		}
		placeholder, createErr := os.CreateTemp(parent, ".noli-backup-*")
		if createErr != nil {
			return problems, fmt.Errorf("generate bundle: reserve backup path: %w", createErr)
		}
		backup = placeholder.Name()
		if closeErr := placeholder.Close(); closeErr != nil {
			_ = os.Remove(backup)
			return problems, fmt.Errorf("generate bundle: close backup placeholder: %w", closeErr)
		}
		if removeErr := os.Remove(backup); removeErr != nil {
			return problems, fmt.Errorf("generate bundle: prepare backup path: %w", removeErr)
		}
		if renameErr := os.Rename(absDestination, backup); renameErr != nil {
			return problems, fmt.Errorf("generate bundle: preserve active bundle: %w", renameErr)
		}
	} else if !os.IsNotExist(statErr) {
		return problems, fmt.Errorf("generate bundle: inspect destination %q: %w", destination, statErr)
	}

	if err := os.Rename(temporary, absDestination); err != nil {
		if backup != "" {
			_ = os.Rename(backup, absDestination)
		}
		return problems, fmt.Errorf("generate bundle: activate generated bundle: %w", err)
	}
	temporaryExists = false
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			return problems, fmt.Errorf("generate bundle: remove replaced bundle backup %q: %w", backup, err)
		}
	}
	return problems, nil
}
