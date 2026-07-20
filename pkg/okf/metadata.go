// Package okf provides the public OKF document model: deterministic Markdown
// frontmatter parsing, bounded bundle loading, typed local links, an immutable
// Store, and standard validation. It adapts documents into the leaf packages
// pkg/graph and pkg/search; those packages never import this one.
package okf

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Metadata is the domain-independent OKF frontmatter model. Extra retains all
// fields that are not part of the small common vocabulary.
type Metadata struct {
	Type        string         `yaml:"type" json:"type"`
	Title       string         `yaml:"title,omitempty" json:"title,omitempty"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Resource    string         `yaml:"resource,omitempty" json:"resource,omitempty"`
	Tags        []string       `yaml:"tags,omitempty" json:"tags,omitempty"`
	Timestamp   string         `yaml:"timestamp,omitempty" json:"timestamp,omitempty"`
	Extra       map[string]any `yaml:",inline" json:"extra,omitempty"`
}

// Clone returns a deep copy; mutating the copy never affects the original.
func (m Metadata) Clone() Metadata {
	clone := m
	if m.Tags != nil {
		clone.Tags = append([]string(nil), m.Tags...)
	}
	if m.Extra != nil {
		clone.Extra = cloneValue(m.Extra).(map[string]any)
	}
	return clone
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = cloneValue(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = cloneValue(item)
		}
		return result
	case []string:
		return append([]string(nil), typed...)
	default:
		return typed
	}
}

func isReservedMetadataKey(key string) bool {
	switch strings.ToLower(key) {
	case "type", "title", "description", "resource", "tags", "timestamp":
		return true
	default:
		return false
	}
}

// MarshalYAML uses a mapping node so common keys remain first and arbitrary
// metadata has a stable lexical order.
func (m Metadata) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendValue := func(key string, value any) error {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
		valueNode := &yaml.Node{}
		if err := valueNode.Encode(value); err != nil {
			return fmt.Errorf("encode metadata field %q: %w", key, err)
		}
		node.Content = append(node.Content, keyNode, valueNode)
		return nil
	}

	if err := appendValue("type", m.Type); err != nil {
		return nil, err
	}
	optional := []struct {
		key   string
		value any
		set   bool
	}{
		{"title", m.Title, m.Title != ""},
		{"description", m.Description, m.Description != ""},
		{"resource", m.Resource, m.Resource != ""},
		{"tags", m.Tags, len(m.Tags) > 0},
		{"timestamp", m.Timestamp, m.Timestamp != ""},
	}
	for _, field := range optional {
		if field.set {
			if err := appendValue(field.key, field.value); err != nil {
				return nil, err
			}
		}
	}

	keys := make([]string, 0, len(m.Extra))
	for key := range m.Extra {
		if !isReservedMetadataKey(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := appendValue(key, m.Extra[key]); err != nil {
			return nil, err
		}
	}
	return node, nil
}

// UnmarshalYAML decodes known fields while retaining every other mapping key.
func (m *Metadata) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("frontmatter must be a YAML mapping")
	}
	*m = Metadata{Extra: make(map[string]any)}
	seen := make(map[string]struct{})
	for i := 0; i < len(node.Content); i += 2 {
		keyNode, valueNode := node.Content[i], node.Content[i+1]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != "!!str" {
			return fmt.Errorf("frontmatter keys must be strings")
		}
		key := keyNode.Value
		lower := strings.ToLower(key)
		if _, ok := seen[lower]; ok {
			return fmt.Errorf("duplicate frontmatter field %q", key)
		}
		seen[lower] = struct{}{}
		switch lower {
		case "type":
			if err := valueNode.Decode(&m.Type); err != nil {
				return fmt.Errorf("decode type: %w", err)
			}
		case "title":
			if err := valueNode.Decode(&m.Title); err != nil {
				return fmt.Errorf("decode title: %w", err)
			}
		case "description":
			if err := valueNode.Decode(&m.Description); err != nil {
				return fmt.Errorf("decode description: %w", err)
			}
		case "resource":
			if err := valueNode.Decode(&m.Resource); err != nil {
				return fmt.Errorf("decode resource: %w", err)
			}
		case "tags":
			if err := valueNode.Decode(&m.Tags); err != nil {
				return fmt.Errorf("decode tags: %w", err)
			}
		case "timestamp":
			if err := valueNode.Decode(&m.Timestamp); err != nil {
				return fmt.Errorf("decode timestamp: %w", err)
			}
		default:
			var value any
			if err := valueNode.Decode(&value); err != nil {
				return fmt.Errorf("decode custom metadata %q: %w", key, err)
			}
			m.Extra[key] = value
		}
	}
	return nil
}
