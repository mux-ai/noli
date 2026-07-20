package project

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func validProfile() ProjectProfile {
	return ProjectProfile{
		Version: 1,
		Project: ProjectInfo{Name: "support", Description: "Source-grounded support"},
		OKF:     OKFSettings{LinkStyle: "root-relative"},
		ConceptTypes: []ConceptTypeConfig{
			{Type: "Product", Directory: "products", IdentityFields: []string{"title", "model"}, Sections: []string{"Overview"}},
			{Type: "Procedure", Directory: "procedures", IdentityFields: []string{"title"}, Sections: []string{"Steps"}, RequiredSections: []string{"Steps"}},
		},
		Relationships: []RelationshipRule{{SourceType: "Procedure", Relation: "applies-to", TargetType: "Product"}},
		Extraction: ExtractionSettings{
			MinimumConfidence: 0.5, RequireSourceEvidence: true, MaximumConceptsPerSource: 20,
		},
	}
}

func TestValidateProfileCollectsDuplicateTypeAndUnsafeDirectory(t *testing.T) {
	profile := validProfile()
	profile.ConceptTypes = append(profile.ConceptTypes, ConceptTypeConfig{
		Type: " product ", Directory: "../outside", IdentityFields: []string{"title"},
	})
	err := ValidateProfile(profile)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("ValidateProfile error = %v, want ValidationError", err)
	}
	joined := err.Error()
	for _, expected := range []string{"duplicates concept type", "directory traversal"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("error %q does not contain %q", joined, expected)
		}
	}
}

func TestValidateProfileRejectsDuplicateDirectoryUnlessAllowed(t *testing.T) {
	profile := validProfile()
	profile.ConceptTypes[1].Directory = "products"
	if err := ValidateProfile(profile); err == nil || !strings.Contains(err.Error(), "duplicates directory") {
		t.Fatalf("ValidateProfile error = %v, want duplicate directory", err)
	}
	profile.OKF.AllowDuplicateDirectories = true
	if err := ValidateProfile(profile); err != nil {
		t.Fatalf("ValidateProfile with shared directories: %v", err)
	}
}

func TestValidateProfileRejectsUnknownRelationshipTypes(t *testing.T) {
	profile := validProfile()
	profile.Relationships[0].TargetType = "Unknown"
	if err := ValidateProfile(profile); err == nil || !strings.Contains(err.Error(), "unknown concept type") {
		t.Fatalf("ValidateProfile error = %v, want unknown concept type", err)
	}
}

func TestNormalizeProfileDeterministic(t *testing.T) {
	profile := validProfile()
	profile.ConceptTypes[0], profile.ConceptTypes[1] = profile.ConceptTypes[1], profile.ConceptTypes[0]
	profile.ConceptTypes[0].IdentityFields = []string{" TITLE ", "title", " Model "}
	first := NormalizeProfile(profile)
	second := NormalizeProfile(first)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("normalization is not idempotent:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if first.ConceptTypes[0].Type != "Procedure" || first.ConceptTypes[0].IdentityFields[0] != "title" {
		t.Fatalf("unexpected normalized concept types: %#v", first.ConceptTypes)
	}
}

func TestProfileJSONStrictRequiredFieldsAndUnknownFields(t *testing.T) {
	profile := validProfile()
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	extraction := object["extraction"].(map[string]any)
	delete(extraction, "require_source_evidence")
	missing, _ := json.Marshal(object)
	if _, err := DecodeProfileJSON(missing); err == nil || !strings.Contains(err.Error(), "require_source_evidence") {
		t.Fatalf("DecodeProfileJSON missing field error = %v", err)
	}
	object["extraction"].(map[string]any)["require_source_evidence"] = true
	object["unexpected"] = true
	unknown, _ := json.Marshal(object)
	if _, err := DecodeProfileJSON(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("DecodeProfileJSON unknown field error = %v", err)
	}
}

func TestSaveLoadProfilesDeterministically(t *testing.T) {
	directory := t.TempDir()
	profile := validProfile()
	jsonPath := filepath.Join(directory, "profile.json")
	yamlPath := filepath.Join(directory, "profile.yaml")
	if err := SaveProfileJSON(jsonPath, profile); err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := os.ReadFile(jsonPath)
	if err := SaveProfileJSON(jsonPath, profile); err != nil {
		t.Fatal(err)
	}
	secondJSON, _ := os.ReadFile(jsonPath)
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("JSON profile output changed between identical saves")
	}
	if err := SaveProfileYAML(yamlPath, profile); err != nil {
		t.Fatal(err)
	}
	jsonProfile, err := LoadProfile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	yamlProfile, err := LoadProfile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(jsonProfile, yamlProfile) {
		t.Fatalf("JSON and YAML profiles differ:\nJSON=%#v\nYAML=%#v", jsonProfile, yamlProfile)
	}
}

func TestInitAndOpenWorkspace(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	workspace, err := InitWorkspace(workspaceRoot, "customer-support")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{workspace.ConfigPath, workspace.InputDir, workspace.NormalizedDir, workspace.KnowledgeDir} {
		if _, err := os.Stat(expected); err != nil {
			t.Errorf("expected workspace path %s: %v", expected, err)
		}
	}
	opened, err := OpenWorkspace(workspaceRoot, "customer-support")
	if err != nil {
		t.Fatal(err)
	}
	if opened.Config.Name != "customer-support" || opened.Root != workspace.Root {
		t.Fatalf("opened workspace = %#v", opened)
	}
	if _, err := ResolveWorkspace(workspaceRoot, "../escape"); err == nil {
		t.Fatal("ResolveWorkspace accepted traversal")
	}
}

type profileFakeClient struct {
	response string
	err      error
}

func (f profileFakeClient) GenerateStructured(_ context.Context, _, _ string, output any) error {
	if f.err != nil {
		return f.err
	}
	raw, ok := output.(*json.RawMessage)
	if !ok {
		return errors.New("unexpected output type")
	}
	*raw = append((*raw)[:0], f.response...)
	return nil
}

func (profileFakeClient) Chat(context.Context, string, string, string) (string, error) {
	return "", errors.New("not used")
}

func TestGenerateProfileStrictlyValidatesLLMResponse(t *testing.T) {
	if _, err := GenerateProfile(context.Background(), profileFakeClient{response: `{bad`}, ProfileRequest{
		ProjectName: "support", Goal: "Create support knowledge",
	}); err == nil || !strings.Contains(err.Error(), "decode profile JSON") {
		t.Fatalf("GenerateProfile malformed response error = %v", err)
	}
	data, err := json.Marshal(validProfile())
	if err != nil {
		t.Fatal(err)
	}
	profile, err := GenerateProfile(context.Background(), profileFakeClient{response: string(data)}, ProfileRequest{
		ProjectName: "support", Goal: "Create support knowledge",
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Project.Name != "support" {
		t.Fatalf("generated profile name = %q", profile.Project.Name)
	}
}
