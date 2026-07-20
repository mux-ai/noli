package okf

import (
	"path/filepath"
	"testing"
)

func todoRules() *ProjectRules {
	return &ProjectRules{
		ConceptTypes: []ConceptTypeRule{
			{Type: "Domain Entity", Directory: "concepts", RequiredMetadata: []string{"title"}},
			{Type: "Business Rule", Directory: "rules", RequiredSections: []string{"Statement"},
				IdentityFields: []string{"title"}},
		},
	}
}

func TestProjectValidationPassesWithoutInventedConfidenceOrCitations(t *testing.T) {
	report := Validate(writeStoreFixture(t), ValidationOptions{Project: todoRules()})
	if !report.Valid {
		t.Fatalf("report = %#v", report)
	}
}

func TestProjectValidationUnknownTypeAndWrongDirectory(t *testing.T) {
	root := writeStoreFixture(t)
	writeTestFile(t, filepath.Join(root, "concepts", "mystery.md"),
		"---\ntype: Mystery\ntitle: M\n---\n\nbody\n")
	writeTestFile(t, filepath.Join(root, "concepts", "misplaced-rule.md"),
		"---\ntype: Business Rule\ntitle: Misplaced\n---\n\n## Statement\n\ntext\n")
	report := Validate(root, ValidationOptions{Project: todoRules()})
	codes := problemCodes(report.Errors)
	if codes[CodeUnknownType] != 1 || codes[CodeWrongDirectory] != 1 {
		t.Fatalf("codes = %#v (%#v)", codes, report.Errors)
	}
}

func TestProjectValidationRequiredMetadataAndSections(t *testing.T) {
	root := writeStoreFixture(t)
	writeTestFile(t, filepath.Join(root, "concepts", "untitled.md"),
		"---\ntype: Domain Entity\n---\n\nbody\n")
	writeTestFile(t, filepath.Join(root, "rules", "no-statement.md"),
		"---\ntype: Business Rule\ntitle: No Statement\n---\n\n## Notes\n\ntext\n")
	rules := todoRules()
	rules.RequiredMetadata = []string{"description"}
	report := Validate(root, ValidationOptions{Project: rules})
	codes := problemCodes(report.Errors)
	if codes[CodeMissingSection] != 1 {
		t.Fatalf("missing section = %d (%#v)", codes[CodeMissingSection], report.Errors)
	}
	// untitled.md misses the type-level "title" requirement; every concept
	// document misses the project-wide "description" requirement.
	if codes[CodeMissingMetadata] < 2 {
		t.Fatalf("missing metadata = %d (%#v)", codes[CodeMissingMetadata], report.Errors)
	}
}

func TestProjectValidationConfidenceAndCitations(t *testing.T) {
	root := writeStoreFixture(t)
	writeTestFile(t, filepath.Join(root, "concepts", "low.md"),
		"---\ntype: Domain Entity\ntitle: Low\nconfidence: 0.2\n---\n\nbody\n")
	rules := todoRules()
	rules.RequireConfidence = true
	rules.MinimumConfidence = 0.5
	rules.RequireCitations = true
	report := Validate(root, ValidationOptions{Project: rules})
	codes := problemCodes(report.Errors)
	if codes[CodeLowConfidence] != 1 {
		t.Fatalf("low confidence = %d (%#v)", codes[CodeLowConfidence], report.Errors)
	}
	// todo-item, task-status, complete-task have no confidence field.
	if codes[CodeMissingConfidence] != 3 {
		t.Fatalf("missing confidence = %d (%#v)", codes[CodeMissingConfidence], report.Errors)
	}
	// No document has a Citations section; low.md counts too.
	if codes[CodeMissingCitation] != 4 {
		t.Fatalf("missing citations = %d (%#v)", codes[CodeMissingCitation], report.Errors)
	}
}

func TestProjectValidationDuplicateIdentityAndUnsafeMetadata(t *testing.T) {
	root := writeStoreFixture(t)
	writeTestFile(t, filepath.Join(root, "rules", "complete-task-copy.md"),
		"---\ntype: Business Rule\ntitle: Complete Task\n---\n\n## Statement\n\ncopy\n")
	writeTestFile(t, filepath.Join(root, "concepts", "unsafe.md"),
		"---\ntype: Domain Entity\ntitle: Unsafe\nresource: javascript:alert(1)\n---\n\nbody\n")
	report := Validate(root, ValidationOptions{Project: todoRules()})
	codes := problemCodes(report.Errors)
	if codes[CodeDuplicateConcept] != 1 {
		t.Fatalf("duplicate concept = %d (%#v)", codes[CodeDuplicateConcept], report.Errors)
	}
	if codes[CodeUnsafeMetadata] != 1 {
		t.Fatalf("unsafe metadata = %d (%#v)", codes[CodeUnsafeMetadata], report.Errors)
	}
}

func TestStandardModeUnaffectedByProjectCodes(t *testing.T) {
	root := writeStoreFixture(t)
	writeTestFile(t, filepath.Join(root, "concepts", "mystery.md"),
		"---\ntype: Mystery\ntitle: M\n---\n\nbody\n")
	report := Validate(root, ValidationOptions{})
	codes := problemCodes(report.Errors)
	if codes[CodeUnknownType] != 0 {
		t.Fatalf("standard mode applied project rules: %#v", report.Errors)
	}
	if !report.Valid {
		t.Fatalf("report = %#v", report)
	}
}
