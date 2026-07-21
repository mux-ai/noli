package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"noli/pkg/generator"
	"noli/pkg/protocol"
)

// runClean implements "clean --dir <repository> [--force]". Without --force
// it only previews: it lists every Noli-related path that would be deleted
// and touches nothing, so agents can show the developer exactly what a
// clean destroys and get explicit confirmation first. --force deletes the
// listed paths. Knowledge is developer-authored content; the two-phase
// contract is what keeps a state command from silently destroying it.
func (a *App) runClean(args []string) int {
	fs := newFlagSet("clean")
	dir := fs.String("dir", ".", "repository directory")
	force := fs.Bool("force", false, "actually delete the listed paths")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("clean", err)
	}
	directory, code := a.checkStateDir("clean", *dir)
	if code != protocol.ExitSuccess {
		return code
	}

	targets := cleanTargets(directory)
	mode := "preview"
	if *force {
		mode = "clean"
		for _, target := range targets {
			if err := os.RemoveAll(filepath.Join(directory, filepath.FromSlash(target))); err != nil {
				return a.failure("clean", protocol.CodeInternalError,
					fmt.Sprintf("delete %q: %v", target, err), nil)
			}
		}
	}
	return a.success("clean", protocol.CleanData{
		Dir:     *dir,
		Mode:    mode,
		Removed: targets,
		Changed: *force && len(targets) > 0,
	})
}

// cleanTargets lists every existing Noli-related path in the repository,
// relative to it: the knowledge root, configuration, state directory,
// prepared queries file, and their deprecated OKF counterparts. Paths come
// from the configuration when it parses; otherwise the frozen defaults.
func cleanTargets(directory string) []string {
	knowledgeRoot := "knowledge"
	queries := ""
	if config, err := generator.LoadConfig(filepath.Join(directory, "noli.yaml")); err == nil {
		knowledgeRoot = config.Knowledge.Root
		queries = config.Agent.QueriesFile
	}
	candidates := []string{
		knowledgeRoot,
		"knowledge",
		"noli.yaml",
		".noli",
		"noli-agent-queries.yaml",
		"okf.yaml",
		".okf",
		"okf-agent-queries.yaml",
	}
	if queries != "" {
		candidates = append(candidates, queries)
	}
	seen := map[string]bool{}
	targets := []string{}
	for _, candidate := range candidates {
		normalized := filepath.ToSlash(filepath.Clean(candidate))
		if normalized == "." || normalized == ".." || seen[normalized] {
			continue
		}
		seen[normalized] = true
		if _, err := os.Lstat(filepath.Join(directory, filepath.FromSlash(normalized))); err == nil {
			targets = append(targets, normalized)
		}
	}
	sort.Strings(targets)
	return targets
}
