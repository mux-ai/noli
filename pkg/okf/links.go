package okf

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"noli/pkg/graph"
)

// PredicateLinksTo is the predicate for ordinary Markdown links.
const PredicateLinksTo = graph.PredicateLinksTo

// recognizedPhrases are the only relationship phrases normalized into typed
// predicates (docs/PROTOCOL.md section 7). Unknown prose stays "links-to".
var recognizedPhrases = []struct {
	phrase    string
	predicate string
}{
	{"applies to", "applies-to"},
	{"enforced by", "enforced-by"},
	{"depends on", "depends-on"},
	{"uses", "uses"},
	{"follows", "follows"},
}

var markdownLinkPattern = regexp.MustCompile(`\[[^\]\n]*\]\(([^)\n]+)\)`)

// ExtractLinks finds local Markdown links in the body and returns typed link
// records sorted by (Target, Predicate) and deduplicated. HTTP(S), other URI
// schemes, images, fragments-only links, and non-Markdown assets are ignored.
// A link on a list-item line whose lead-in matches a recognized relationship
// phrase (for example "- Applies to: [X](x.md)") receives that predicate;
// every other link uses "links-to".
func ExtractLinks(root, documentPath, body string) ([]Link, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve knowledge root %q: %w", root, err)
	}
	currentRel := filepath.Clean(filepath.FromSlash(documentPath))
	if filepath.IsAbs(currentRel) || escapesRoot(currentRel) {
		return nil, fmt.Errorf("document path %q escapes knowledge root", documentPath)
	}

	seen := make(map[Link]struct{})
	links := make([]Link, 0)
	for _, line := range strings.Split(body, "\n") {
		predicate := linePredicate(line)
		for _, match := range markdownLinkPattern.FindAllStringSubmatchIndex(line, -1) {
			if match[0] > 0 && line[match[0]-1] == '!' {
				continue // image
			}
			raw := strings.TrimSpace(line[match[2]:match[3]])
			target, resolveErr := resolveLinkTarget(absRoot, currentRel, raw)
			if resolveErr != nil {
				return nil, resolveErr
			}
			if target == "" {
				continue
			}
			link := Link{Target: target, Predicate: predicate}
			if _, exists := seen[link]; exists {
				continue
			}
			seen[link] = struct{}{}
			links = append(links, link)
		}
	}
	sort.Slice(links, func(i, j int) bool {
		if links[i].Target != links[j].Target {
			return links[i].Target < links[j].Target
		}
		return links[i].Predicate < links[j].Predicate
	})
	return links, nil
}

// resolveLinkTarget normalizes one link destination into a document ID, or ""
// when the destination is external, an asset, or a pure fragment.
func resolveLinkTarget(absRoot, currentRel, raw string) (string, error) {
	destination := markdownDestination(raw)
	if destination == "" || strings.ContainsRune(destination, '\x00') {
		return "", nil
	}
	parsed, err := url.Parse(destination)
	if err != nil {
		return "", fmt.Errorf("parse link destination %q: %w", destination, err)
	}
	if parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(destination, "//") {
		return "", nil
	}
	decodedPath, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		return "", fmt.Errorf("decode link destination %q: %w", destination, err)
	}
	if decodedPath == "" {
		return "", nil
	}
	if strings.Contains(decodedPath, "\\") {
		return "", fmt.Errorf("link destination %q contains a backslash", destination)
	}

	var target string
	if rooted, ok := strings.CutPrefix(decodedPath, "/"); ok {
		target = filepath.Join(absRoot, filepath.FromSlash(rooted))
	} else {
		target = filepath.Join(absRoot, filepath.Dir(currentRel), filepath.FromSlash(decodedPath))
	}
	if strings.HasSuffix(decodedPath, "/") {
		target = filepath.Join(target, "index.md")
	}
	target = filepath.Clean(target)
	rel, err := filepath.Rel(absRoot, target)
	if err != nil {
		return "", fmt.Errorf("resolve link destination %q: %w", destination, err)
	}
	if escapesRoot(rel) {
		return "", fmt.Errorf("link destination %q escapes knowledge root", destination)
	}
	ext := filepath.Ext(rel)
	if ext != "" && !strings.EqualFold(ext, ".md") {
		return "", nil
	}
	id := strings.TrimSuffix(filepath.ToSlash(rel), ext)
	if id == "." || id == "" {
		return "", nil
	}
	return id, nil
}

// linePredicate returns the typed predicate for links on this line, or
// "links-to". A typed line is a list item that starts with a recognized
// phrase, followed by an optional colon and then a Markdown link.
func linePredicate(line string) string {
	trimmed := strings.TrimSpace(line)
	rest := ""
	for _, marker := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(trimmed, marker) {
			rest = strings.TrimSpace(trimmed[len(marker):])
			break
		}
	}
	if rest == "" {
		return PredicateLinksTo
	}
	lower := strings.ToLower(rest)
	for _, candidate := range recognizedPhrases {
		if !strings.HasPrefix(lower, candidate.phrase) {
			continue
		}
		tail := rest[len(candidate.phrase):]
		if tail == "" {
			continue
		}
		if tail[0] != ':' && tail[0] != ' ' && tail[0] != '\t' {
			continue // phrase is a prefix of a longer word
		}
		tail = strings.TrimSpace(tail)
		tail = strings.TrimSpace(strings.TrimPrefix(tail, ":"))
		if strings.HasPrefix(tail, "[") {
			return candidate.predicate
		}
	}
	return PredicateLinksTo
}

func markdownDestination(raw string) string {
	if strings.HasPrefix(raw, "<") {
		if end := strings.Index(raw, ">"); end > 0 {
			return raw[1:end]
		}
	}
	for i, r := range raw {
		if r == ' ' || r == '\t' {
			return raw[:i]
		}
	}
	return raw
}
