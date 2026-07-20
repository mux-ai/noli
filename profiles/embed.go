// Package profiles provides the predefined, demonstration-only extraction
// profiles shipped with Noli.
package profiles

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"noli/internal/project"
)

//go:embed *.yaml
var files embed.FS

// Names returns the supported predefined profile names in lexical order.
func Names() []string {
	names := []string{"credit-card", "product-support", "software-project"}
	sort.Strings(names)
	return names
}

// Load strictly parses, normalizes, and validates a predefined profile.
func Load(name string) (project.ProjectProfile, error) {
	name = strings.TrimSpace(name)
	switch name {
	case "credit-card", "product-support", "software-project":
	default:
		return project.ProjectProfile{}, fmt.Errorf("load predefined profile %q: unknown profile (available: %s)", name, strings.Join(Names(), ", "))
	}
	data, err := files.ReadFile(name + ".yaml")
	if err != nil {
		return project.ProjectProfile{}, fmt.Errorf("load predefined profile %q: %w", name, err)
	}
	var profile project.ProjectProfile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&profile); err != nil {
		return project.ProjectProfile{}, fmt.Errorf("decode predefined profile %q: %w", name, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return project.ProjectProfile{}, fmt.Errorf("decode predefined profile %q: multiple YAML documents", name)
		}
		return project.ProjectProfile{}, fmt.Errorf("decode predefined profile %q trailing content: %w", name, err)
	}
	profile = project.ApplyDefaults(project.NormalizeProfile(profile))
	if err := project.ValidateProfile(profile); err != nil {
		return project.ProjectProfile{}, fmt.Errorf("validate predefined profile %q: %w", name, err)
	}
	return profile, nil
}
