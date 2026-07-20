package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"noli/internal/project"
	"noli/internal/source"
)

type fakeClient struct {
	calls int
}

func (f *fakeClient) GenerateStructured(_ context.Context, _ string, userPrompt string, output any) error {
	f.calls++
	if !strings.Contains(userPrompt, `"output_schema"`) {
		return fmt.Errorf("prompt omitted output schema")
	}
	response, ok := output.(*ConceptResponse)
	if !ok {
		return fmt.Errorf("unexpected output type %T", output)
	}
	var prompt struct {
		SourceExcerpts []SourceExcerpt `json:"source_excerpts"`
	}
	if err := json.Unmarshal([]byte(userPrompt), &prompt); err != nil {
		return err
	}
	if len(prompt.SourceExcerpts) != 1 {
		return fmt.Errorf("got %d excerpts", len(prompt.SourceExcerpts))
	}
	excerpt := prompt.SourceExcerpts[0]
	response.Concepts = []ConceptDraft{{
		TemporaryID: "concept", Type: "item", Title: fmt.Sprintf("Part %d", f.calls), Description: "Extracted part",
		Sections:  []SectionDraft{{Heading: "Summary", Content: excerpt.Content}},
		Citations: []Citation{{SourceID: excerpt.SourceID, Evidence: excerpt.Content}}, Confidence: .9,
	}}
	return nil
}

func (*fakeClient) Chat(context.Context, string, string, string) (string, error) {
	return "", nil
}

func TestExtractorChunksAndCanonicalizesTypeAlias(t *testing.T) {
	client := &fakeClient{}
	extractor, err := NewExtractor(client, Settings{MaximumChunkCharacters: 100, MaximumSourceExcerpts: 4, MaximumOutputConcepts: 3})
	if err != nil {
		t.Fatal(err)
	}
	profile := project.ProjectProfile{
		ConceptTypes: []project.ConceptTypeConfig{{Type: "Item", Aliases: []string{"item"}, Directory: "items", IdentityFields: []string{"title"}}},
		Extraction: project.ExtractionSettings{
			MinimumConfidence: .5, RequireSourceEvidence: true, MaximumConceptsPerSource: 5,
			MaximumChunkCharacters: 10, MaximumSourceExcerpts: 1,
		},
	}
	documents := []source.SourceDocument{{ID: "source-1", Name: "one.txt", SourceURI: "input/one.txt", Content: "1234567890abcdefghij"}}
	concepts, err := extractor.Extract(context.Background(), profile, documents)
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 2 || len(concepts) != 2 {
		t.Fatalf("calls = %d, concepts = %d", client.calls, len(concepts))
	}
	if concepts[0].Type != "Item" || concepts[1].Type != "Item" {
		t.Fatalf("types were not canonicalized: %#v", concepts)
	}
	if concepts[0].TemporaryID == concepts[1].TemporaryID {
		t.Fatalf("temporary IDs were not made unique: %#v", concepts)
	}
}
