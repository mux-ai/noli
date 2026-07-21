package cli

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"noli/pkg/generator"
	"noli/pkg/protocol"
)

// runDrift implements "drift --config noli.yaml". It is read-only and
// reports two drift classes: the active knowledge diverging from its
// rendered concept source (hand edits), and repository files changed since
// the last knowledge-touching commit without a knowledge update
// (undocumented changes). A drifted project stays a success envelope and
// carries exit code 4, mirroring "validate".
func (a *App) runDrift(args []string) int {
	fs := newFlagSet("drift")
	configPath := fs.String("config", "", "noli.yaml path")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("drift", err)
	}
	if err := requireValue("--config", *configPath); err != nil {
		return a.invalidArgument("drift", err)
	}
	config, err := generator.LoadConfig(*configPath)
	if err != nil {
		return a.configFailure("drift", *configPath, err)
	}
	diff, err := generator.Diff(config)
	if err != nil {
		switch {
		case errors.Is(err, generator.ErrUnsafePath):
			return a.failure("drift", protocol.CodeUnsafePath, err.Error(),
				map[string]string{"config": *configPath})
		default:
			return a.failure("drift", protocol.CodeGenerationFailed, err.Error(),
				map[string]string{"config": *configPath})
		}
	}
	bundle := protocol.DriftBundleData{
		InSync:  len(diff.Added) == 0 && len(diff.Changed) == 0 && len(diff.Removed) == 0,
		Added:   diff.Added,
		Changed: diff.Changed,
		Removed: diff.Removed,
	}
	git, baseline, files := scanUndocumentedChanges(config)
	data := protocol.DriftData{
		Config:            *configPath,
		Bundle:            bundle,
		Git:               git,
		Baseline:          baseline,
		UndocumentedFiles: files,
		Drifted:           !bundle.InSync || len(files) > 0,
	}
	exit := a.success("drift", data)
	if exit == protocol.ExitSuccess && data.Drifted {
		return protocol.ExitValidation
	}
	return exit
}

// scanUndocumentedChanges finds repository files changed after the last
// commit that touched the knowledge root or the concept sources, plus all
// uncommitted changes, excluding the knowledge paths themselves. Without a
// usable git repository it degrades to git "unavailable" and an empty list.
func scanUndocumentedChanges(config *generator.Config) (git, baseline string, files []protocol.DriftFileData) {
	files = []protocol.DriftFileData{}
	repoRoot, err := gitOutput(config.Dir(), "rev-parse", "--show-toplevel")
	if err != nil || repoRoot == "" {
		return "unavailable", "", files
	}

	knowledgePaths := knowledgeRelativePaths(config, repoRoot)
	if len(knowledgePaths) == 0 {
		return "unavailable", "", files
	}
	projectRel, err := repoRelative(repoRoot, config.Dir())
	if err != nil {
		return "unavailable", "", files
	}

	baseline, _ = gitOutput(repoRoot, append(
		[]string{"log", "-1", "--format=%H", "--"}, knowledgePaths...)...)

	states := map[string]string{}
	if baseline != "" {
		committed, err := gitOutput(repoRoot, "diff", "--name-status", baseline, "HEAD")
		if err == nil {
			collectNameStatus(committed, states)
		}
	}
	uncommitted, err := gitOutput(repoRoot, "status", "--porcelain")
	if err == nil {
		collectPorcelain(uncommitted, states)
	}

	paths := make([]string, 0, len(states))
	for path := range states {
		if isDocumentedPath(path, projectRel, knowledgePaths) {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		files = append(files, protocol.DriftFileData{Path: path, State: states[path]})
	}
	return "available", baseline, files
}

// knowledgeRelativePaths lists the repository-relative paths that count as
// knowledge updates: the knowledge root, every concept file, the .noli
// directory, and noli.yaml itself.
func knowledgeRelativePaths(config *generator.Config, repoRoot string) []string {
	candidates := []string{
		filepath.Join(config.Dir(), filepath.FromSlash(config.Knowledge.Root)),
		filepath.Join(config.Dir(), ".noli"),
		filepath.Join(config.Dir(), "noli.yaml"),
		filepath.Join(config.Dir(), "okf.yaml"),
	}
	for _, conceptFile := range config.Generation.ConceptFiles {
		candidates = append(candidates, filepath.Join(config.Dir(), filepath.FromSlash(conceptFile)))
	}
	paths := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, candidate := range candidates {
		rel, err := repoRelative(repoRoot, candidate)
		if err != nil || seen[rel] {
			continue
		}
		seen[rel] = true
		paths = append(paths, rel)
	}
	return paths
}

// isDocumentedPath reports whether a repository-relative file is either a
// knowledge path itself or outside the project directory.
func isDocumentedPath(path, projectRel string, knowledgePaths []string) bool {
	if projectRel != "." && path != projectRel && !strings.HasPrefix(path, projectRel+"/") {
		return true
	}
	for _, knowledge := range knowledgePaths {
		if path == knowledge || strings.HasPrefix(path, knowledge+"/") {
			return true
		}
	}
	return false
}

// collectNameStatus parses "git diff --name-status" lines into path states.
func collectNameStatus(output string, states map[string]string) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] == "" {
			continue
		}
		path := fields[len(fields)-1]
		states[path] = diffStateName(fields[0][:1])
	}
}

// collectPorcelain parses "git status --porcelain" lines into path states;
// uncommitted states win over committed ones.
func collectPorcelain(output string, states map[string]string) {
	for _, line := range strings.Split(output, "\n") {
		if len(line) < 4 {
			continue
		}
		status, path := line[:2], line[3:]
		if index := strings.Index(path, " -> "); index >= 0 {
			path = path[index+4:]
		}
		if status == "??" {
			states[path] = "untracked"
			continue
		}
		letter := strings.TrimSpace(string(status[0]))
		if letter == "" {
			letter = string(status[1])
		}
		states[path] = diffStateName(letter)
	}
}

func diffStateName(letter string) string {
	switch letter {
	case "A":
		return "added"
	case "D":
		return "deleted"
	case "R":
		return "renamed"
	case "C":
		return "copied"
	default:
		return "modified"
	}
}

// repoRelative converts an absolute path into the slash-separated form git
// uses relative to the repository root, resolving symlinked ancestors so
// tmp-dir repositories compare correctly.
func repoRelative(repoRoot, target string) (string, error) {
	resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return "", err
	}
	resolvedTarget := target
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		resolvedTarget = resolved
	} else if resolvedParent, parentErr := filepath.EvalSymlinks(filepath.Dir(target)); parentErr == nil {
		resolvedTarget = filepath.Join(resolvedParent, filepath.Base(target))
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("target is outside the repository")
	}
	return filepath.ToSlash(rel), nil
}

// gitOutput runs one git command and returns its stdout with only trailing
// newlines trimmed: porcelain status lines start with a significant space.
// Stderr is discarded; any failure surfaces only as the error.
func gitOutput(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return "", err
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}
