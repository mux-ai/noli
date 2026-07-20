package okf

// Link is a typed local link record. Target is a normalized document ID
// (root-relative slash path without the .md extension). Predicate is either
// a recognized relationship predicate or "links-to".
type Link struct {
	Target    string `json:"target"`
	Predicate string `json:"predicate"`
}

// Document is a parsed OKF Markdown document.
type Document struct {
	// ID is the root-relative slash path without the .md extension.
	ID string `json:"id"`
	// Path is the root-relative slash path including the extension.
	Path     string   `json:"path"`
	Metadata Metadata `json:"metadata"`
	Body     string   `json:"body"`
	Links    []Link   `json:"links"`
	// IsIndex marks index.md navigation documents at any depth.
	IsIndex bool `json:"is_index"`
	// IsLog marks the root log.md document.
	IsLog bool `json:"is_log"`
}

// Clone returns a deep copy; mutating the copy never affects the original.
func (d Document) Clone() Document {
	clone := d
	clone.Metadata = d.Metadata.Clone()
	if d.Links != nil {
		clone.Links = append([]Link(nil), d.Links...)
	}
	return clone
}

// LinkTargets returns the deduplicated link target IDs in link order.
func (d Document) LinkTargets() []string {
	targets := make([]string, 0, len(d.Links))
	seen := make(map[string]struct{}, len(d.Links))
	for _, link := range d.Links {
		if _, exists := seen[link.Target]; exists {
			continue
		}
		seen[link.Target] = struct{}{}
		targets = append(targets, link.Target)
	}
	return targets
}
