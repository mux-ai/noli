package okf

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"noli/internal/project"
)

func TestRenderConceptStableYAMLSectionsAndLinks(t *testing.T) {
	concept := Concept{
		ID: "guides/wifi", Type: "Guide", Title: "Wi-Fi [help]", Description: "Resolve Wi-Fi.",
		Tags: []string{"network", "Network", "wifi"}, Confidence: 0.9,
		Attributes: map[string]any{"zeta": true, "alpha": []string{"x"}},
		Sections: []Section{
			{Heading: "Resolution", Content: "Restart."},
			{Heading: "Symptoms", Content: "Offline."},
			{Heading: "Symptoms", Content: "Offline."},
		},
		Citations: []Citation{{SourceID: "manual", Page: 2}},
	}
	relations := []Relation{{From: concept.ID, To: "products/x100", Predicate: "Applies to", Confidence: 1}}
	options := RenderOptions{Timestamp: "2026-07-20T10:00:00+08:00", SectionOrder: map[string][]string{"Guide": {"Symptoms", "Resolution"}}}
	first, err := RenderConcept(concept, relations, map[string]string{"products/x100": "X100 [Pro]"}, options)
	if err != nil {
		t.Fatalf("RenderConcept() error = %v", err)
	}
	second, err := RenderConcept(concept, relations, map[string]string{"products/x100": "X100 [Pro]"}, options)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("rendered Markdown is not stable")
	}
	text := string(first)
	if strings.Index(text, "alpha:") > strings.Index(text, "zeta:") {
		t.Fatalf("custom YAML fields are not sorted:\n%s", text)
	}
	if strings.Count(text, "# Symptoms") != 1 {
		t.Fatalf("duplicate heading was rendered:\n%s", text)
	}
	if strings.Index(text, "# Symptoms") > strings.Index(text, "# Resolution") {
		t.Fatalf("profile section order not respected:\n%s", text)
	}
	if !strings.Contains(text, `[X100 \[Pro\]](/products/x100.md)`) {
		t.Fatalf("safe relation link missing:\n%s", text)
	}
}

func TestRenderIndexes(t *testing.T) {
	directory := DirectoryIndex{Directory: "guides", Type: "Guide", Entries: []IndexEntry{
		{ID: "guides/z", Title: "Zulu", Description: "Last"},
		{ID: "guides/a", Title: "Alpha", Description: "First"},
	}}
	options := RenderOptions{Timestamp: "2026-07-20T10:00:00+08:00"}
	directoryMarkdown, err := RenderDirectoryIndex(directory, options)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(string(directoryMarkdown), "Alpha") > strings.Index(string(directoryMarkdown), "Zulu") {
		t.Fatalf("directory index order is unstable:\n%s", directoryMarkdown)
	}
	rootMarkdown, err := RenderRootIndex("Support", "Source-backed help.", []DirectoryIndex{directory}, options)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rootMarkdown), "[Guide](/guides/index.md) — 2 documents") {
		t.Fatalf("root index missing directory count:\n%s", rootMarkdown)
	}
}

func TestRenderAndValidateBundle(t *testing.T) {
	profile := testProfile()
	root := t.TempDir()
	concepts := []Concept{{
		ID: "guides/wifi", Type: "Guide", Title: "Wi-Fi", Description: "Help", Confidence: 0.9,
		Sections:  []Section{{Heading: "Resolution", Content: "Restart."}},
		Citations: []Citation{{SourceID: "manual", Page: 1}},
	}}
	run := RunInfo{GeneratedAt: time.Date(2026, 7, 20, 10, 0, 0, 0, time.FixedZone("SGT", 8*60*60)), Profile: "test", SourceDocuments: 1, ExtractedDrafts: 1}
	if err := RenderBundle(root, profile, concepts, nil, run); err != nil {
		t.Fatalf("RenderBundle() error = %v", err)
	}
	for _, name := range []string{"index.md", "log.md", "guides/index.md", "guides/wifi.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	problems := ValidateBundle(root, ProjectMode, ValidationOptions{Profile: &profile})
	if err := ProblemsError(problems); err != nil {
		t.Fatalf("ValidateBundle() = %v; problems = %#v", err, problems)
	}
}

func TestGenerateBundleLeavesActiveBundleOnFailure(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "knowledge")
	writeTestFile(t, filepath.Join(destination, "marker"), "active")
	profile := testProfile()
	concepts := []Concept{{
		ID: "guides/wifi", Type: "Guide", Title: "Wi-Fi", Confidence: 0.9,
		Attributes: map[string]any{"type": "collision"}, Sections: []Section{{Heading: "Resolution", Content: "Restart."}},
	}}
	run := RunInfo{GeneratedAt: time.Now()}
	if _, err := GenerateBundle(destination, profile, concepts, nil, run, false); err == nil {
		t.Fatal("GenerateBundle() unexpectedly succeeded")
	}
	data, err := os.ReadFile(filepath.Join(destination, "marker"))
	if err != nil || string(data) != "active" {
		t.Fatalf("active bundle was changed: data=%q err=%v", data, err)
	}
}

func testProfile() project.ProjectProfile {
	return project.ProjectProfile{
		Version: 1,
		Project: project.ProjectInfo{Name: "Support"},
		OKF:     project.OKFSettings{Title: "Support", Description: "Source-backed help.", LinkStyle: "root-relative"},
		ConceptTypes: []project.ConceptTypeConfig{{
			Type: "Guide", Directory: "guides", IdentityFields: []string{"title"},
			Sections: []string{"Resolution"}, RequiredSections: []string{"Resolution"}, RequiredFields: []string{"title"},
		}},
		Extraction: project.ExtractionSettings{MinimumConfidence: 0.5, RequireSourceEvidence: true, MaximumConceptsPerSource: 10},
	}
}
