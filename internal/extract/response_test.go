package extract

import (
	"strings"
	"testing"

	"noli/internal/project"
)

func testProfile() project.ProjectProfile {
	return project.ProjectProfile{
		ConceptTypes: []project.ConceptTypeConfig{{Type: "Product", Directory: "products", IdentityFields: []string{"title"}, RequiredFields: []string{"model"}}},
		Extraction:   project.ExtractionSettings{MinimumConfidence: 0.5, RequireSourceEvidence: true, MaximumConceptsPerSource: 10},
	}
}

func TestDecodeConceptResponseRejectsMalformedJSON(t *testing.T) {
	if _, err := DecodeConceptResponse([]byte(`{"concepts":[}`)); err == nil {
		t.Fatal("expected malformed JSON error")
	}
	if _, err := DecodeConceptResponse([]byte(`{"concepts":[],"markdown":"# no"}`)); err == nil {
		t.Fatal("expected unknown field error")
	}
	if _, err := DecodeConceptResponse([]byte(`{}`)); err == nil {
		t.Fatal("expected missing concepts error")
	}
}

func TestValidateConceptsAggregatesProblems(t *testing.T) {
	concept := ConceptDraft{
		TemporaryID: "one", Type: "Product", Title: "Widget", Description: "A widget", Confidence: 1.2,
		Sections:  []SectionDraft{{Heading: "Summary", Content: "text"}},
		Citations: []Citation{{SourceID: "missing"}},
	}
	err := ValidateConcepts([]ConceptDraft{concept}, testProfile(), map[string]struct{}{"source": {}})
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, expected := range []string{"confidence", "required field", "unknown"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("error %q does not contain %q", err, expected)
		}
	}
}

func TestValidateConceptsAcceptsArbitraryAttributes(t *testing.T) {
	concept := ConceptDraft{
		TemporaryID: "one", Type: "Product", Title: "Widget", Description: "A widget", Confidence: 0.9,
		Attributes: map[string]any{"model": "X100", "domain_specific": map[string]any{"any": true}},
		Sections:   []SectionDraft{{Heading: "Summary", Content: "text"}},
		Citations:  []Citation{{SourceID: "source", Evidence: "Widget model X100"}},
	}
	if err := ValidateConcepts([]ConceptDraft{concept}, testProfile(), map[string]struct{}{"source": {}}); err != nil {
		t.Fatal(err)
	}
}
