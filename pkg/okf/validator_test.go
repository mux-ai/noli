package okf

import (
	"path/filepath"
	"sort"
	"testing"
)

func problemCodes(problems []Problem) map[string]int {
	codes := make(map[string]int)
	for _, problem := range problems {
		codes[problem.Code]++
	}
	return codes
}

func TestValidateCleanBundle(t *testing.T) {
	report := Validate(writeStoreFixture(t), ValidationOptions{})
	if !report.Valid {
		t.Fatalf("report = %#v", report)
	}
	if report.Errors == nil || report.Warnings == nil {
		t.Fatal("report arrays must be non-nil")
	}
	if len(report.Errors) != 0 || len(report.Warnings) != 0 {
		t.Fatalf("unexpected problems: %#v", report)
	}
}

// OKF v0.1 section 9: broken cross-links and missing index files are
// reported but must never fail a bundle in standard mode. Only the missing
// type (conformance rule 2) is an error.
func TestValidateStandardModeDoesNotRejectForSpecToleratedProblems(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "rules", "broken.md"), `---
type: Business Rule
title: Broken
---

[one](../concepts/missing-one.md)
[two](../concepts/missing-two.md)
`)
	report := Validate(root, ValidationOptions{})
	if !report.Valid {
		t.Fatalf("standard mode rejected a spec-tolerated bundle: %#v", report.Errors)
	}
	warnings := problemCodes(report.Warnings)
	if warnings[CodeBrokenLink] != 2 {
		t.Fatalf("broken links = %d, want 2 warnings (%#v)", warnings[CodeBrokenLink], report.Warnings)
	}
	// Root index and rules/index are both absent.
	if warnings[CodeMissingIndex] != 2 {
		t.Fatalf("missing index = %d (%#v)", warnings[CodeMissingIndex], report.Warnings)
	}

	// A missing type stays an error: conformance rule 2 is a MUST.
	writeTestFile(t, filepath.Join(root, "untyped.md"), "---\ntitle: No Type\n---\n\nbody\n")
	typed := Validate(root, ValidationOptions{})
	if typed.Valid {
		t.Fatal("missing type did not fail the bundle")
	}
	if problemCodes(typed.Errors)[CodeMissingType] != 1 {
		t.Fatalf("missing type = %#v", typed.Errors)
	}
	if !sort.SliceIsSorted(typed.Errors, func(i, j int) bool {
		left, right := typed.Errors[i], typed.Errors[j]
		if left.Document != right.Document {
			return left.Document < right.Document
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		return left.Message < right.Message
	}) {
		t.Fatalf("errors are not in stable order: %#v", typed.Errors)
	}
}

// Project mode is opt-in local policy and may escalate the same structural
// problems to errors.
func TestValidateProjectModeEscalatesStructuralProblems(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "rules", "broken.md"), `---
type: Business Rule
title: Broken
---

[one](../concepts/missing-one.md)
`)
	report := Validate(root, ValidationOptions{Project: &ProjectRules{
		ConceptTypes: []ConceptTypeRule{{Type: "Business Rule", Directory: "rules"}},
	}})
	if report.Valid {
		t.Fatal("project mode accepted a broken bundle")
	}
	codes := problemCodes(report.Errors)
	if codes[CodeBrokenLink] != 1 || codes[CodeMissingIndex] != 2 {
		t.Fatalf("project codes = %#v (%#v)", codes, report.Errors)
	}
}

// OKF v0.1 conformance rule 1 exempts reserved filenames from the
// frontmatter requirement; section 6 forbids frontmatter on index files.
func TestValidateAcceptsFrontmatterFreeReservedFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.md"),
		"# My Project\n\n* [Widget](/concepts/widget.md) - a widget\n")
	writeTestFile(t, filepath.Join(root, "log.md"),
		"# Log\n\n## 2026-07-20\n\n**Creation** Initial bundle.\n")
	writeTestFile(t, filepath.Join(root, "concepts", "index.md"),
		"# Concepts\n\n* [Widget](/concepts/widget.md) - a widget\n")
	writeTestFile(t, filepath.Join(root, "concepts", "widget.md"),
		"---\ntype: Domain Entity\ntitle: Widget\ncustom_key: preserved\n---\n\n## Definition\n\nA widget.\n")
	report := Validate(root, ValidationOptions{})
	if !report.Valid || len(report.Warnings) != 0 {
		t.Fatalf("conformant bundle rejected: %#v", report)
	}

	store, err := Load(root)
	if err != nil {
		t.Fatalf("conformant bundle failed to load: %v", err)
	}
	index, ok := store.Get("index")
	if !ok || !index.IsIndex || index.Metadata.Type != "" {
		t.Fatalf("index document = %#v", index)
	}
	log, ok := store.Get("log")
	if !ok || !log.IsLog {
		t.Fatalf("log document = %#v", log)
	}
	widget, ok := store.Get("concepts/widget")
	if !ok || widget.Metadata.Extra["custom_key"] != "preserved" {
		t.Fatalf("unknown frontmatter key was not preserved: %#v", widget)
	}
}

// Section 7 requires ISO 8601 date headings in logs; a malformed log is a
// warning, never a rejection.
func TestValidateLogHeadingsWarnOnly(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "log.md"), "# Log\n\n## July 2026\n\nprose\n")
	report := Validate(root, ValidationOptions{})
	if !report.Valid {
		t.Fatalf("malformed log rejected the bundle: %#v", report.Errors)
	}
	if problemCodes(report.Warnings)[CodeInvalidLogHeading] != 1 {
		t.Fatalf("warnings = %#v", report.Warnings)
	}
}

func TestValidateConfidenceRules(t *testing.T) {
	root := writeStoreFixture(t)
	writeTestFile(t, filepath.Join(root, "concepts", "bad-confidence.md"), `---
type: Domain Entity
title: Bad
confidence: 2.5
---

body
`)
	writeTestFile(t, filepath.Join(root, "concepts", "text-confidence.md"), `---
type: Domain Entity
title: Text
confidence: high
---

body
`)
	report := Validate(root, ValidationOptions{})
	codes := problemCodes(report.Errors)
	if codes[CodeInvalidConfidence] != 2 {
		t.Fatalf("invalid confidence = %d (%#v)", codes[CodeInvalidConfidence], report.Errors)
	}
	if codes[CodeMissingConfidence] != 0 {
		t.Fatal("confidence was required without RequireConfidence")
	}

	required := Validate(root, ValidationOptions{RequireConfidence: true})
	requiredCodes := problemCodes(required.Errors)
	// Every concept without confidence: todo-item, task-status, complete-task.
	if requiredCodes[CodeMissingConfidence] != 3 {
		t.Fatalf("missing confidence = %d (%#v)", requiredCodes[CodeMissingConfidence], required.Errors)
	}
}

// Reserved files that still carry frontmatter (older producers) are parsed
// and tolerated; no type requirement applies to them.
func TestValidateToleratesReservedFilesWithFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.md"), "---\ntype: Navigation\n---\n\ncontent\n")
	writeTestFile(t, filepath.Join(root, "log.md"), "---\ntype: Bundle Log\n---\n\n## 2026-07-20\n\nentry\n")
	report := Validate(root, ValidationOptions{})
	if !report.Valid || len(report.Warnings) != 0 {
		t.Fatalf("report = %#v", report)
	}
	store, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if index, _ := store.Get("index"); index.Metadata.Type != "Navigation" {
		t.Fatalf("frontmatter on a reserved file was dropped: %#v", index)
	}
}

func TestValidateEmptyBodyWarnsAndParseProblemsAggregate(t *testing.T) {
	root := writeStoreFixture(t)
	writeTestFile(t, filepath.Join(root, "concepts", "empty.md"), "---\ntype: Domain Entity\ntitle: Empty\n---\n")
	writeTestFile(t, filepath.Join(root, "concepts", "bad.md"), "no frontmatter\n")
	report := Validate(root, ValidationOptions{})
	if report.Valid {
		t.Fatal("bundle with parse problems reported valid")
	}
	codes := problemCodes(report.Errors)
	if codes[CodeParseError] != 1 {
		t.Fatalf("parse errors = %#v", codes)
	}
	warningCodes := problemCodes(report.Warnings)
	if warningCodes[CodeEmptyDocument] != 1 {
		t.Fatalf("warnings = %#v", report.Warnings)
	}
}

func TestValidateMissingRootIsBundleLevelError(t *testing.T) {
	report := Validate(filepath.Join(t.TempDir(), "missing"), ValidationOptions{})
	if report.Valid || len(report.Errors) != 1 || report.Errors[0].Code != CodeParseError || report.Errors[0].Document != "" {
		t.Fatalf("report = %#v", report)
	}
}
