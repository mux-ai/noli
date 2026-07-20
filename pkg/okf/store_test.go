package okf

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeStoreFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.md"), `---
type: Navigation
title: Home
---

- [Concepts](concepts/)
`)
	writeTestFile(t, filepath.Join(root, "log.md"), `---
type: Bundle Log
title: Log
---

## 2026-07-20
`)
	writeTestFile(t, filepath.Join(root, "concepts", "index.md"), `---
type: Navigation
title: Concepts
---

- [Todo Item](todo-item.md)
`)
	writeTestFile(t, filepath.Join(root, "concepts", "todo-item.md"), `---
type: Domain Entity
title: Todo Item
tags: [domain]
custom:
  owner: docs
---

## Definition

- Uses: [Status](task-status.md)
`)
	writeTestFile(t, filepath.Join(root, "concepts", "task-status.md"), `---
type: Domain Entity
title: Task Status
---

## States
`)
	writeTestFile(t, filepath.Join(root, "rules", "index.md"), `---
type: Navigation
title: Rules
---

- [Complete Task](complete-task.md)
`)
	writeTestFile(t, filepath.Join(root, "rules", "complete-task.md"), `---
type: Business Rule
title: Complete Task
---

## Statement

- Applies to: [Todo Item](../concepts/todo-item.md)
`)
	return root
}

func TestLoadBuildsImmutableStore(t *testing.T) {
	store, err := Load(writeStoreFixture(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if store.DocumentCount() != 7 {
		t.Fatalf("DocumentCount() = %d", store.DocumentCount())
	}
	if store.BundleID() == "" {
		t.Fatal("BundleID is empty")
	}
	document, ok := store.Get("concepts/todo-item")
	if !ok || document.Metadata.Title != "Todo Item" {
		t.Fatalf("Get() = %#v, %v", document, ok)
	}
	wantLinks := []Link{{Target: "concepts/task-status", Predicate: "uses"}}
	if !reflect.DeepEqual(document.Links, wantLinks) {
		t.Fatalf("Links = %#v, want %#v", document.Links, wantLinks)
	}

	// Mutating the returned copy must not corrupt the Store.
	document.Metadata.Title = "Mutated"
	document.Metadata.Extra["custom"].(map[string]any)["owner"] = "mutated"
	document.Links[0].Target = "mutated"
	fresh, _ := store.Get("concepts/todo-item")
	if fresh.Metadata.Title != "Todo Item" || fresh.Links[0].Target != "concepts/task-status" {
		t.Fatalf("Store was mutated through Get copy: %#v", fresh)
	}
	if fresh.Metadata.Extra["custom"].(map[string]any)["owner"] != "docs" {
		t.Fatal("nested Extra was mutated through Get copy")
	}

	if _, ok := store.Get("missing"); ok {
		t.Fatal("Get(missing) reported ok")
	}
}

func TestListFiltersAndOrder(t *testing.T) {
	store, err := Load(writeStoreFixture(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	defaults := store.List(ListOptions{})
	gotIDs := make([]string, len(defaults))
	for i, document := range defaults {
		gotIDs[i] = document.ID
	}
	want := []string{"concepts/task-status", "concepts/todo-item", "rules/complete-task"}
	if !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("List() = %#v, want %#v", gotIDs, want)
	}
	typed := store.List(ListOptions{Types: []string{"business rule"}})
	if len(typed) != 1 || typed[0].ID != "rules/complete-task" {
		t.Fatalf("typed List() = %#v", typed)
	}
	withNavigation := store.List(ListOptions{IncludeNavigation: true, IncludeLog: true})
	if len(withNavigation) != 7 {
		t.Fatalf("inclusive List() length = %d", len(withNavigation))
	}
}

func TestLoadFailsOnParseProblems(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bad.md"), "no frontmatter\n")
	if _, err := Load(root); err == nil {
		t.Fatal("Load succeeded on a broken bundle")
	}
	if _, err := Load(filepath.Join(root, "missing-root")); err == nil {
		t.Fatal("Load succeeded on a missing root")
	}
}

func TestNilStoreAccessorsAreSafe(t *testing.T) {
	var store *Store
	if store.DocumentCount() != 0 || store.Root() != "" || store.BundleID() != "" {
		t.Fatal("nil Store reported content")
	}
	if ids := store.IDs(); ids == nil || len(ids) != 0 {
		t.Fatalf("nil IDs() = %#v", ids)
	}
	if list := store.List(ListOptions{}); list == nil || len(list) != 0 {
		t.Fatalf("nil List() = %#v", list)
	}
	if _, ok := store.Get("x"); ok {
		t.Fatal("nil Get() reported ok")
	}
}

func TestLoadRespectsParseOptions(t *testing.T) {
	root := writeStoreFixture(t)
	writeTestFile(t, filepath.Join(root, "drafts", "wip.md"), conceptDocument("WIP"))
	store, err := LoadWithOptions(root, ParseOptions{Exclude: []string{"drafts"}})
	if err != nil {
		t.Fatalf("LoadWithOptions() error = %v", err)
	}
	if _, ok := store.Get("drafts/wip"); ok {
		t.Fatal("excluded document was loaded")
	}
	if err := os.WriteFile(filepath.Join(root, "drafts", "wip.md"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadWithOptions(root, ParseOptions{Exclude: []string{"drafts"}}); err != nil {
		t.Fatalf("exclusion did not shield broken file: %v", err)
	}
}
