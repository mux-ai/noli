package studio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"noli/internal/llm"
	"noli/internal/project"
	"noli/internal/source"
	"noli/profiles"
)

type ProfileOptions struct {
	Selection   string
	Goal        string
	Description string
	Hints       []string
}

// ProfileWorkspace generates or loads a profile, adapts it to the workspace
// project identity, validates it, and records it as the active staging profile.
func ProfileWorkspace(ctx context.Context, client llm.Client, workspace project.Workspace, options ProfileOptions) (project.ProjectProfile, []source.Warning, error) {
	selection := strings.TrimSpace(options.Selection)
	if selection == "" {
		selection = strings.TrimSpace(workspace.Config.Profile)
	}
	if selection == "" {
		selection = "auto"
	}
	goal := firstNonEmpty(options.Goal, workspace.Config.Goal)
	description := firstNonEmpty(options.Description, workspace.Config.Description)
	hints := mergeStrings(workspace.Config.Hints, options.Hints)

	var (
		profile  project.ProjectProfile
		warnings []source.Warning
		err      error
	)
	if selection == "auto" {
		if client == nil {
			return project.ProjectProfile{}, nil, fmt.Errorf("profile workspace %s automatically: LLM client is required", workspace.Config.Name)
		}
		if strings.TrimSpace(goal) == "" {
			return project.ProjectProfile{}, nil, fmt.Errorf("profile workspace %s automatically: project goal is required", workspace.Config.Name)
		}
		loader := source.NewLoader()
		loaded, loadErr := loader.LoadDirectory(ctx, workspace.InputDir)
		if loadErr != nil {
			return project.ProjectProfile{}, nil, fmt.Errorf("profile workspace %s: load representative sources: %w", workspace.Config.Name, loadErr)
		}
		warnings = loaded.Warnings
		profile, err = project.GenerateProfile(ctx, client, project.ProfileRequest{
			ProjectName:        workspace.Config.Name,
			ProjectDescription: description,
			Goal:               goal,
			Hints:              hints,
			SourceExcerpts:     representativeExcerpts(loaded.Documents),
			MaximumExcerpts:    project.DefaultMaximumSourceExcerpts,
			MaximumExcerptSize: 4000,
		})
		if err != nil {
			return project.ProjectProfile{}, warnings, fmt.Errorf("profile workspace %s: %w", workspace.Config.Name, err)
		}
	} else if isPredefinedProfile(selection) {
		profile, err = profiles.Load(selection)
		if err != nil {
			return project.ProjectProfile{}, nil, fmt.Errorf("profile workspace %s: %w", workspace.Config.Name, err)
		}
	} else {
		profile, err = project.LoadProfile(selection)
		if err != nil {
			return project.ProjectProfile{}, nil, fmt.Errorf("profile workspace %s from %s: %w", workspace.Config.Name, selection, err)
		}
	}

	profile.Project.Name = workspace.Config.Name
	if description != "" {
		profile.Project.Description = description
	}
	if goal != "" {
		profile.Project.Goal = goal
	}
	profile.Project.Hints = mergeStrings(profile.Project.Hints, hints)
	profile = project.ApplyDefaults(project.NormalizeProfile(profile))
	if err := project.ValidateProfile(profile); err != nil {
		return project.ProjectProfile{}, warnings, fmt.Errorf("profile workspace %s: validate active profile: %w", workspace.Config.Name, err)
	}
	if err := project.SaveProfileJSON(workspace.GeneratedProfilePath, profile); err != nil {
		return project.ProjectProfile{}, warnings, fmt.Errorf("profile workspace %s: %w", workspace.Config.Name, err)
	}
	workspace.Config.Profile = selection
	workspace.Config.Goal = goal
	workspace.Config.Description = description
	workspace.Config.Hints = hints
	if err := project.SaveConfig(workspace.ConfigPath, workspace.Config); err != nil {
		return project.ProjectProfile{}, warnings, fmt.Errorf("profile workspace %s: update project config: %w", workspace.Config.Name, err)
	}
	return profile, warnings, nil
}

// ActiveProfile loads the reviewed staging profile first, then falls back to a
// configured predefined or custom profile when no staging profile exists.
func ActiveProfile(workspace project.Workspace) (project.ProjectProfile, error) {
	profile, err := project.LoadProfile(workspace.GeneratedProfilePath)
	if err == nil {
		return profile, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return project.ProjectProfile{}, fmt.Errorf("load active profile for %s: %w", workspace.Config.Name, err)
	}
	selection := strings.TrimSpace(workspace.Config.Profile)
	if selection == "" || selection == "auto" {
		return project.ProjectProfile{}, fmt.Errorf("load active profile for %s: no generated profile; run the profile command first", workspace.Config.Name)
	}
	if isPredefinedProfile(selection) {
		profile, err = profiles.Load(selection)
	} else {
		profile, err = project.LoadProfile(selection)
	}
	if err != nil {
		return project.ProjectProfile{}, fmt.Errorf("load active profile for %s: %w", workspace.Config.Name, err)
	}
	profile.Project.Name = workspace.Config.Name
	if workspace.Config.Description != "" {
		profile.Project.Description = workspace.Config.Description
	}
	if workspace.Config.Goal != "" {
		profile.Project.Goal = workspace.Config.Goal
	}
	profile.Project.Hints = mergeStrings(profile.Project.Hints, workspace.Config.Hints)
	profile = project.ApplyDefaults(project.NormalizeProfile(profile))
	if err := project.ValidateProfile(profile); err != nil {
		return project.ProjectProfile{}, fmt.Errorf("load active profile for %s: %w", workspace.Config.Name, err)
	}
	return profile, nil
}

func representativeExcerpts(documents []source.SourceDocument) []project.SourceExcerpt {
	sorted := append([]source.SourceDocument(nil), documents...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	result := make([]project.SourceExcerpt, 0, len(sorted))
	for _, document := range sorted {
		content := strings.TrimSpace(document.Content)
		if content == "" {
			continue
		}
		result = append(result, project.SourceExcerpt{SourceID: document.ID, Name: document.Name, URI: document.SourceURI, Content: content})
	}
	return result
}

func isPredefinedProfile(name string) bool {
	for _, candidate := range profiles.Names() {
		if name == candidate {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func mergeStrings(groups ...[]string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, group := range groups {
		for _, value := range group {
			value = strings.TrimSpace(value)
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
	}
	return result
}
