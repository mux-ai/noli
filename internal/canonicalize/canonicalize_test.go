package canonicalize

import (
	"reflect"
	"testing"

	"noli/internal/extract"
	"noli/internal/project"
)

func canonicalTestProfile() project.ProjectProfile {
	return project.ProjectProfile{
		ConceptTypes: []project.ConceptTypeConfig{
			{Type: "Product", Directory: "products", IdentityFields: []string{"title", "model"}, Sections: []string{"Summary"}},
			{Type: "Guide", Directory: "guides", IdentityFields: []string{"title", "model"}, Sections: []string{"Symptoms", "Resolution"}},
		},
		Relationships: []project.RelationshipRule{{SourceType: "Guide", Relation: "applies to", TargetType: "Product"}},
		Extraction:    project.ExtractionSettings{MinimumConfidence: 0.5},
	}
}

func TestSlugDeterministicUnicodeAndEmpty(t *testing.T) {
	for input, expected := range map[string]string{
		" Wi-Fi  Failure! ": "wi-fi-failure",
		"Crème brûlée":      "crème-brûlée",
		"产品 100":            "产品-100",
	} {
		actual, err := Slug(input)
		if err != nil {
			t.Fatalf("Slug(%q): %v", input, err)
		}
		if actual != expected {
			t.Errorf("Slug(%q) = %q, want %q", input, actual, expected)
		}
	}
	if _, err := Slug("../"); err == nil {
		t.Fatal("expected empty slug rejection")
	}
}

func TestDuplicateCandidateAndExactMerge(t *testing.T) {
	profile := canonicalTestProfile()
	drafts := []extract.ConceptDraft{
		{TemporaryID: "b", Type: "Product", Title: "Router", Description: "Home router", Confidence: .8, Tags: []string{"WiFi"}, Attributes: map[string]any{"model": "X100"}, Sections: []extract.SectionDraft{{Heading: "Summary", Content: "Fast."}}},
		{TemporaryID: "a", Type: "Product", Title: " Router ", Description: "Home router", Confidence: .9, Tags: []string{"wifi"}, Attributes: map[string]any{"model": "x100"}, Sections: []extract.SectionDraft{{Heading: "Summary", Content: "Fast."}}},
	}
	groups := FindDuplicateCandidates(profile, drafts)
	if len(groups) != 1 || !reflect.DeepEqual(groups[0].TemporaryIDs, []string{"a", "b"}) {
		t.Fatalf("groups = %#v", groups)
	}
	concepts, err := Canonicalize(profile, drafts)
	if err != nil {
		t.Fatal(err)
	}
	if len(concepts) != 1 || concepts[0].ID != "products/router" {
		t.Fatalf("concepts = %#v", concepts)
	}
	if len(concepts[0].Tags) != 1 || concepts[0].Tags[0] != "wifi" || concepts[0].Conflicts != nil {
		t.Fatalf("merge was not exact: %#v", concepts[0])
	}
}

func TestCanonicalizationPreservesConflicts(t *testing.T) {
	profile := canonicalTestProfile()
	drafts := []extract.ConceptDraft{
		{TemporaryID: "a", Type: "Product", Title: "Router", Description: "First", Confidence: .9, Attributes: map[string]any{"model": "X100", "color": "red"}},
		{TemporaryID: "b", Type: "Product", Title: "Router", Description: "Second", Confidence: .8, Attributes: map[string]any{"model": "x100", "color": "blue"}},
	}
	concepts, err := Canonicalize(profile, drafts)
	if err != nil {
		t.Fatal(err)
	}
	if len(concepts[0].Conflicts["description"]) != 2 || len(concepts[0].Conflicts["attributes.color"]) != 2 {
		t.Fatalf("conflicts = %#v", concepts[0].Conflicts)
	}
}

func TestDeterministicIdentityCollisionHandling(t *testing.T) {
	profile := canonicalTestProfile()
	drafts := []extract.ConceptDraft{
		{TemporaryID: "x200", Type: "Guide", Title: "WiFi Failure", Description: "X200", Confidence: .9, Attributes: map[string]any{"model": "X200"}},
		{TemporaryID: "x100", Type: "Guide", Title: "WiFi Failure", Description: "X100", Confidence: .9, Attributes: map[string]any{"model": "X100"}},
	}
	first, err := Canonicalize(profile, drafts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Canonicalize(profile, []extract.ConceptDraft{drafts[1], drafts[0]})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("order changed output:\n%#v\n%#v", first, second)
	}
	if first[0].ID != "guides/wifi-failure-x100" || first[1].ID != "guides/wifi-failure-x200" {
		t.Fatalf("IDs = %q, %q", first[0].ID, first[1].ID)
	}
}

func TestRelationshipResolutionAndUnresolved(t *testing.T) {
	profile := canonicalTestProfile()
	concepts := []CanonicalConcept{
		{ID: "products/x100", Type: "Product", Title: "X100 Router", Attributes: map[string]any{"model": "X100"}},
		{ID: "products/x200", Type: "Product", Title: "X200 Router", Attributes: map[string]any{"model": "X200"}},
		{ID: "guides/wifi", Type: "Guide", Title: "WiFi failure", Relations: []extract.RelationDraft{
			{Predicate: "applies to", TargetReference: "X100", Confidence: .9},
			{Predicate: "applies to", TargetReference: "Router", Confidence: .9},
		}},
	}
	edges, unresolved := ResolveRelationships(profile, concepts)
	if len(edges) != 1 || edges[0].To != "products/x100" {
		t.Fatalf("edges = %#v", edges)
	}
	if len(unresolved) != 1 || unresolved[0].TargetReference != "Router" {
		t.Fatalf("unresolved = %#v", unresolved)
	}
}
