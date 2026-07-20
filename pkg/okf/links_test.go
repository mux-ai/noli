package okf

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractLinksResolutionAndFiltering(t *testing.T) {
	root := t.TempDir()
	body := `[relative](../products/x100.md#details)
[root](/policies/refund.md#exceptions)
[external](https://example.com/x.md)
[mail](mailto:help@example.com)
![image](../images/x.md)
[asset](manual.pdf)
[anchor](#local)`
	links, err := ExtractLinks(root, "troubleshooting/wifi.md", body)
	if err != nil {
		t.Fatalf("ExtractLinks() error = %v", err)
	}
	want := []Link{
		{Target: "policies/refund", Predicate: "links-to"},
		{Target: "products/x100", Predicate: "links-to"},
	}
	if !reflect.DeepEqual(links, want) {
		t.Fatalf("links = %#v, want %#v", links, want)
	}
}

func TestExtractLinksRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, destination := range []string{"../../../secret.md", "%2e%2e/%2e%2e/secret.md"} {
		_, err := ExtractLinks(root, "docs/item.md", "[bad]("+destination+")")
		if err == nil || !strings.Contains(err.Error(), "escapes knowledge root") {
			t.Fatalf("destination %q error = %v", destination, err)
		}
	}
}

func TestExtractLinksRecognizedPredicates(t *testing.T) {
	root := t.TempDir()
	body := `- Applies to: [Todo Item](../concepts/todo-item.md)
* Enforced by [Service](../components/todo-service.md)
- depends on: [Status](../concepts/task-status.md)
- Uses: [Repo](../components/todo-repository.md)
- Follows [Flow](../workflows/create-task.md)
- Useless [Not Typed](../concepts/not-typed.md)
- Related reading: [Prose](../concepts/prose.md)
Plain [Inline](../concepts/inline.md) link.`
	links, err := ExtractLinks(root, "rules/complete-task.md", body)
	if err != nil {
		t.Fatalf("ExtractLinks() error = %v", err)
	}
	want := []Link{
		{Target: "components/todo-repository", Predicate: "uses"},
		{Target: "components/todo-service", Predicate: "enforced-by"},
		{Target: "concepts/inline", Predicate: "links-to"},
		{Target: "concepts/not-typed", Predicate: "links-to"},
		{Target: "concepts/prose", Predicate: "links-to"},
		{Target: "concepts/task-status", Predicate: "depends-on"},
		{Target: "concepts/todo-item", Predicate: "applies-to"},
		{Target: "workflows/create-task", Predicate: "follows"},
	}
	if !reflect.DeepEqual(links, want) {
		t.Fatalf("links = %#v, want %#v", links, want)
	}
}

func TestExtractLinksDeduplicatesByTargetAndPredicate(t *testing.T) {
	root := t.TempDir()
	body := `[one](a.md)
[again](a.md)
- Uses: [typed](a.md)`
	links, err := ExtractLinks(root, "doc.md", body)
	if err != nil {
		t.Fatalf("ExtractLinks() error = %v", err)
	}
	want := []Link{
		{Target: "a", Predicate: "links-to"},
		{Target: "a", Predicate: "uses"},
	}
	if !reflect.DeepEqual(links, want) {
		t.Fatalf("links = %#v, want %#v", links, want)
	}
}

func TestExtractLinksDirectoryLinkResolvesToIndex(t *testing.T) {
	root := t.TempDir()
	links, err := ExtractLinks(root, "index.md", "[concepts](concepts/)")
	if err != nil {
		t.Fatalf("ExtractLinks() error = %v", err)
	}
	want := []Link{{Target: "concepts/index", Predicate: "links-to"}}
	if !reflect.DeepEqual(links, want) {
		t.Fatalf("links = %#v, want %#v", links, want)
	}
}
