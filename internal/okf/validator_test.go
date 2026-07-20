package okf

import (
	"path/filepath"
	"testing"
)

func TestStandardsValidationReportsMissingTypeAndStructure(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.md"), "---\ntype: Navigation\ntitle: Home\n---\n\n# Home\n")
	writeTestFile(t, filepath.Join(root, "notes", "item.md"), "---\ntitle: Item\n---\n\n# Body\n")
	problems := ValidateBundle(root, StandardMode, ValidationOptions{})
	codes := make(map[string]bool)
	for _, problem := range problems {
		codes[problem.Code] = true
	}
	for _, code := range []string{"missing-type", "missing-bundle-log", "missing-directory-index"} {
		if !codes[code] {
			t.Errorf("missing validation problem %q: %#v", code, problems)
		}
	}
}

func TestProjectValidationAggregatesProblemsAndWarnings(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.md"), "---\ntype: Navigation\ntitle: Home\n---\n\n# Home\n")
	writeTestFile(t, filepath.Join(root, "log.md"), "---\ntype: Bundle Log\ntitle: Log\n---\n\n# Log\n")
	writeTestFile(t, filepath.Join(root, "guides", "index.md"), "---\ntype: Navigation\ntitle: Guide\n---\n\n# Guide\n")
	writeTestFile(t, filepath.Join(root, "guides", "bad.md"), `---
type: Guide
title: Bad
confidence: 0.1
---

# Notes

[missing](/guides/nope.md)
`)
	profile := testProfile()
	problems := ValidateBundle(root, ProjectMode, ValidationOptions{Profile: &profile, UnresolvedRelations: 2})
	codes := make(map[string]Severity)
	for _, problem := range problems {
		codes[problem.Code] = problem.Severity
	}
	for _, code := range []string{"low-confidence", "missing-required-section", "missing-citations", "unresolved-link"} {
		if codes[code] != SeverityError {
			t.Errorf("problem %q missing or not error: %#v", code, problems)
		}
	}
	if codes["unresolved-relationships"] != SeverityWarning {
		t.Errorf("unresolved relationship should be warning: %#v", problems)
	}
	if ProblemsError(problems) == nil {
		t.Fatal("ProblemsError() unexpectedly returned nil")
	}
}

func TestProjectDuplicateValidationUsesConfiguredIdentityFields(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.md"), "---\ntype: Navigation\ntitle: Home\n---\n\n# Home\n")
	writeTestFile(t, filepath.Join(root, "log.md"), "---\ntype: Bundle Log\ntitle: Log\n---\n\n# Log\n")
	writeTestFile(t, filepath.Join(root, "guides", "index.md"), "---\ntype: Navigation\ntitle: Guides\n---\n\n# Guides\n")
	for name, model := range map[string]string{"one": "X100", "two": "X200"} {
		writeTestFile(t, filepath.Join(root, "guides", name+".md"), "---\ntype: Guide\ntitle: Wi-Fi help\nmodel: "+model+"\nconfidence: 0.9\n---\n\n# Resolution\n\nRestart.\n\n# Citations\n\n- manual.\n")
	}
	profile := testProfile()
	profile.ConceptTypes[0].IdentityFields = []string{"title", "model"}
	problems := ValidateBundle(root, ProjectMode, ValidationOptions{Profile: &profile})
	for _, problem := range problems {
		if problem.Code == "duplicate-concept" {
			t.Fatalf("different configured identities were marked duplicate: %#v", problems)
		}
	}
}
