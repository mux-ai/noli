// Package studiocli implements Noli's extended ingestion and extraction CLI.
package studiocli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"noli/internal/llm"
	"noli/internal/okf"
	"noli/internal/project"
	"noli/internal/studio"
)

// Run dispatches one extended Noli CLI invocation.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		writeUsage(stdout)
		return nil
	}
	runtimeConfig, err := studio.RuntimeConfigFromEnv()
	if err != nil {
		return err
	}
	command := args[0]
	commandArgs := args[1:]
	switch command {
	case "help", "-h", "--help":
		writeUsage(stdout)
		return nil
	case "version", "--version":
		fmt.Fprintln(stdout, okf.GeneratorVersion)
		return nil
	case "init":
		return runInit(runtimeConfig, commandArgs, stdout)
	case "import":
		return runImport(runtimeConfig, commandArgs, stdout, stderr)
	case "profile":
		return runProfile(ctx, runtimeConfig, commandArgs, stdout, stderr)
	case "profile-show":
		return runProfileShow(runtimeConfig, commandArgs, stdout)
	case "extract":
		return runExtract(ctx, runtimeConfig, commandArgs, stdout, stderr)
	case "generate":
		return runGenerate(ctx, runtimeConfig, commandArgs, stdout, stderr)
	case "validate":
		return runValidate(runtimeConfig, commandArgs, stdout)
	case "list":
		return runList(runtimeConfig, commandArgs, stdout)
	case "search":
		return runSearch(runtimeConfig, commandArgs, stdout)
	case "graph":
		return runGraph(runtimeConfig, commandArgs, stdout)
	case "ask":
		return runAsk(ctx, runtimeConfig, commandArgs, stdout)
	case "export":
		return runExport(runtimeConfig, commandArgs, stdout)
	default:
		return fmt.Errorf("unknown command %q; run noligen help", command)
	}
}

func runInit(config studio.RuntimeConfig, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: noligen init <project>")
	}
	workspace, err := project.InitWorkspace(config.WorkspacePath, args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Initialized %s\n", workspace.Root)
	return nil
}

func runImport(config studio.RuntimeConfig, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: noligen import <project> <path> [path ...]")
	}
	workspace, err := project.OpenWorkspace(config.WorkspacePath, args[0])
	if err != nil {
		return err
	}
	imported, warnings, err := studio.ImportPaths(workspace.InputDir, args[1:])
	for _, warning := range warnings {
		fmt.Fprintln(stderr, "warning:", warning)
	}
	if err != nil {
		return err
	}
	for _, filename := range imported {
		fmt.Fprintln(stdout, filename)
	}
	fmt.Fprintf(stdout, "Imported %d source files\n", len(imported))
	return nil
}

func runProfile(ctx context.Context, config studio.RuntimeConfig, args []string, stdout, stderr io.Writer) error {
	projectName, remaining, err := leadingProject(args)
	if err != nil {
		return fmt.Errorf("usage: noligen profile <project> --goal <goal> --profile <auto|name|file>")
	}
	flags := newFlagSet("profile")
	goal := flags.String("goal", "", "project goal")
	selection := flags.String("profile", "", "auto, a predefined profile, or a profile file")
	description := flags.String("description", "", "project description")
	var hints stringListFlag
	flags.Var(&hints, "hint", "optional extraction hint; may be repeated")
	if err := flags.Parse(remaining); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("profile: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	workspace, err := project.OpenWorkspace(config.WorkspacePath, projectName)
	if err != nil {
		return err
	}
	chosen := strings.TrimSpace(*selection)
	if chosen == "" {
		chosen = workspace.Config.Profile
	}
	var client llm.Client
	if chosen == "" || chosen == "auto" {
		client, err = ollamaClient(config)
		if err != nil {
			return err
		}
	}
	profile, warnings, err := studio.ProfileWorkspace(ctx, client, workspace, studio.ProfileOptions{
		Selection: chosen, Goal: *goal, Description: *description, Hints: hints,
	})
	for _, warning := range warnings {
		fmt.Fprintln(stderr, "warning:", warning.String())
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Active profile: %s (%d concept types)\n", profile.Project.Name, len(profile.ConceptTypes))
	fmt.Fprintf(stdout, "Saved %s\n", workspace.GeneratedProfilePath)
	return nil
}

func runProfileShow(config studio.RuntimeConfig, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: noligen profile-show <project>")
	}
	workspace, err := project.OpenWorkspace(config.WorkspacePath, args[0])
	if err != nil {
		return err
	}
	profile, err := studio.ActiveProfile(workspace)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("show active profile: %w", err)
	}
	fmt.Fprintln(stdout, string(data))
	return nil
}

func runExtract(ctx context.Context, config studio.RuntimeConfig, args []string, stdout, stderr io.Writer) error {
	projectName, remaining, err := leadingProject(args)
	if err != nil {
		return fmt.Errorf("usage: noligen extract <project>")
	}
	flags := newFlagSet("extract")
	if err := flags.Parse(remaining); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("extract: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	workspace, profile, err := openWorkspaceAndProfile(config, projectName)
	if err != nil {
		return err
	}
	client, err := ollamaClient(config)
	if err != nil {
		return err
	}
	engine := configuredEngine(client, config)
	result, err := engine.ExtractWorkspace(ctx, workspace, profile)
	for _, warning := range result.Warnings {
		fmt.Fprintln(stderr, "warning:", warning.String())
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Extracted %d concept drafts from %d source documents\n", len(result.Drafts), len(result.Documents))
	fmt.Fprintf(stdout, "Saved %s\n", workspace.ConceptDraftsPath)
	return nil
}

func runGenerate(ctx context.Context, config studio.RuntimeConfig, args []string, stdout, stderr io.Writer) error {
	projectName := ""
	remaining := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		projectName = args[0]
		remaining = args[1:]
	}
	flags := newFlagSet("generate")
	input := flags.String("input", "", "source input directory for one-command generation")
	goal := flags.String("goal", "", "project goal for one-command generation")
	output := flags.String("output", "", "knowledge output directory for one-command generation")
	selection := flags.String("profile", "", "auto, predefined profile, or profile file")
	strict := flags.Bool("strict", false, "fail when relationships cannot be resolved")
	if err := flags.Parse(remaining); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("generate: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if projectName == "" {
		return runDirectGenerate(ctx, config, directGenerateOptions{
			Input: *input, Goal: *goal, Output: *output, Profile: *selection, Strict: *strict,
		}, stdout, stderr)
	}
	if *input != "" || *output != "" || *goal != "" {
		return fmt.Errorf("generate: --input, --goal, and --output are only valid without a workspace project name")
	}
	workspace, profile, err := openWorkspaceAndProfile(config, projectName)
	if err != nil {
		return err
	}
	client, err := ollamaClient(config)
	if err != nil {
		return err
	}
	engine := configuredEngine(client, config)
	engine.StrictRelationships = *strict
	result, err := engine.GenerateWorkspace(ctx, workspace, profile)
	for _, warning := range result.SourceWarnings {
		fmt.Fprintln(stderr, "warning:", warning.String())
	}
	for _, problem := range result.ValidationProblems {
		if problem.Severity == okf.SeverityWarning {
			fmt.Fprintln(stderr, "warning:", problem.Error())
		}
	}
	if err != nil {
		return err
	}
	printGenerationSummary(stdout, workspace, result)
	return nil
}

type directGenerateOptions struct {
	Input   string
	Goal    string
	Output  string
	Profile string
	Strict  bool
}

func runDirectGenerate(ctx context.Context, config studio.RuntimeConfig, options directGenerateOptions, stdout, stderr io.Writer) (returnErr error) {
	if strings.TrimSpace(options.Input) == "" || strings.TrimSpace(options.Goal) == "" || strings.TrimSpace(options.Output) == "" {
		return fmt.Errorf("usage: noligen generate --input <directory> --goal <goal> --output <directory> --profile <auto|name|file>")
	}
	output, err := safeOutputDestination(options.Output)
	if err != nil {
		return err
	}
	parent := filepath.Dir(output)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("one-command generation: create output parent %s: %w", parent, err)
	}
	temporaryRoot, err := os.MkdirTemp(parent, ".noli-studio-direct-*")
	if err != nil {
		return fmt.Errorf("one-command generation: create temporary workspace: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(temporaryRoot); removeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("one-command generation: clean temporary workspace: %w", removeErr)
		}
	}()
	workspace, err := project.InitWorkspace(temporaryRoot, "direct-project")
	if err != nil {
		return err
	}
	workspace.Config.Goal = strings.TrimSpace(options.Goal)
	workspace.Config.Profile = firstCLIValue(options.Profile, "auto")
	if err := project.SaveConfig(workspace.ConfigPath, workspace.Config); err != nil {
		return err
	}
	if _, warnings, err := studio.ImportPaths(workspace.InputDir, []string{options.Input}); err != nil {
		return err
	} else {
		for _, warning := range warnings {
			fmt.Fprintln(stderr, "warning:", warning)
		}
	}
	client, err := ollamaClient(config)
	if err != nil {
		return err
	}
	profile, profileWarnings, err := studio.ProfileWorkspace(ctx, client, workspace, studio.ProfileOptions{
		Selection: workspace.Config.Profile, Goal: workspace.Config.Goal,
	})
	for _, warning := range profileWarnings {
		fmt.Fprintln(stderr, "warning:", warning.String())
	}
	if err != nil {
		return err
	}
	workspace, err = project.LoadWorkspace(workspace.Root)
	if err != nil {
		return err
	}
	workspace.KnowledgeDir = output
	engine := configuredEngine(client, config)
	engine.StrictRelationships = options.Strict
	result, err := engine.GenerateWorkspace(ctx, workspace, profile)
	if err != nil {
		return err
	}
	printGenerationSummary(stdout, workspace, result)
	return nil
}

func runValidate(config studio.RuntimeConfig, args []string, stdout io.Writer) error {
	projectName, remaining, err := leadingProject(args)
	if err != nil {
		return fmt.Errorf("usage: noligen validate <project> --mode <standard|project>")
	}
	flags := newFlagSet("validate")
	modeText := flags.String("mode", string(okf.StandardMode), "standard or project")
	strict := flags.Bool("strict", false, "treat unresolved relationships as errors")
	if err := flags.Parse(remaining); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("validate: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	workspace, err := project.OpenWorkspace(config.WorkspacePath, projectName)
	if err != nil {
		return err
	}
	mode := okf.ValidationMode(strings.ToLower(strings.TrimSpace(*modeText)))
	options := okf.ValidationOptions{StrictRelationships: *strict, RequireBundleLog: true, RequireDirectoryIndexes: true}
	if mode == okf.ProjectMode {
		profile, err := studio.ActiveProfile(workspace)
		if err != nil {
			return err
		}
		options.Profile = &profile
		unresolved, loadErr := studio.LoadUnresolvedRelations(workspace.UnresolvedRelationsPath)
		if loadErr == nil {
			options.UnresolvedRelations = len(unresolved)
		} else if !errors.Is(loadErr, os.ErrNotExist) {
			return loadErr
		}
	}
	problems := okf.ValidateBundle(workspace.KnowledgeDir, mode, options)
	for _, problem := range problems {
		fmt.Fprintln(stdout, problem.Error())
	}
	if err := okf.ProblemsError(problems); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Valid %s bundle (%d warnings)\n", mode, warningCount(problems))
	return nil
}

func runList(config studio.RuntimeConfig, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: noligen list <project>")
	}
	workspace, err := project.OpenWorkspace(config.WorkspacePath, args[0])
	if err != nil {
		return err
	}
	bundle, err := okf.ParseBundle(workspace.KnowledgeDir)
	if err != nil {
		return err
	}
	count := 0
	for _, id := range bundle.Order {
		document := bundle.Documents[id]
		if document.IsIndex || document.IsLog {
			continue
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", id, document.Metadata.Type, document.Metadata.Title)
		count++
	}
	fmt.Fprintf(stdout, "%d concepts\n", count)
	return nil
}

func runSearch(config studio.RuntimeConfig, args []string, stdout io.Writer) error {
	projectName, remaining, err := leadingProject(args)
	if err != nil {
		return fmt.Errorf("usage: noligen search <project> [--limit N] <query>")
	}
	flags := newFlagSet("search")
	limit := flags.Int("limit", 10, "maximum result count")
	if err := flags.Parse(remaining); err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if query == "" {
		return fmt.Errorf("search: query must not be empty")
	}
	workspace, err := project.OpenWorkspace(config.WorkspacePath, projectName)
	if err != nil {
		return err
	}
	loaded, err := studio.LoadStore(workspace, activeProfileIfAvailable(workspace))
	if err != nil {
		return err
	}
	results := loaded.Search(query, *limit)
	for _, result := range results {
		fmt.Fprintf(stdout, "%d\t%s\t%s\t%s\n", result.Score, result.ID, result.Type, result.Title)
	}
	fmt.Fprintf(stdout, "%d results\n", len(results))
	return nil
}

func runGraph(config studio.RuntimeConfig, args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: noligen graph <project> <document-id>")
	}
	workspace, err := project.OpenWorkspace(config.WorkspacePath, args[0])
	if err != nil {
		return err
	}
	loaded, err := studio.LoadStore(workspace, activeProfileIfAvailable(workspace))
	if err != nil {
		return err
	}
	id := strings.TrimSuffix(strings.TrimPrefix(args[1], "/"), ".md")
	if !loaded.Graph.HasNode(id) {
		return fmt.Errorf("graph: unknown document ID %q", id)
	}
	fmt.Fprintln(stdout, "OUTGOING")
	for _, edge := range loaded.Graph.Outgoing(id) {
		fmt.Fprintf(stdout, "%s\t%s\n", edge.Predicate, edge.To)
	}
	fmt.Fprintln(stdout, "INCOMING")
	for _, edge := range loaded.Graph.Incoming(id) {
		fmt.Fprintf(stdout, "%s\t%s\n", edge.Predicate, edge.From)
	}
	return nil
}

func runAsk(ctx context.Context, config studio.RuntimeConfig, args []string, stdout io.Writer) error {
	projectName, remaining, err := leadingProject(args)
	if err != nil {
		return fmt.Errorf("usage: noligen ask <project> [--context-limit N] <question>")
	}
	flags := newFlagSet("ask")
	contextLimit := flags.Int("context-limit", config.ContextLimit, "maximum retrieved context characters")
	if err := flags.Parse(remaining); err != nil {
		return err
	}
	question := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if question == "" {
		return fmt.Errorf("ask: question must not be empty")
	}
	workspace, profile, err := openWorkspaceAndProfile(config, projectName)
	if err != nil {
		return err
	}
	client, err := ollamaClient(config)
	if err != nil {
		return err
	}
	result, err := studio.Ask(ctx, client, workspace, profile, question, *contextLimit)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, result.Answer)
	if len(result.SelectedIDs) > 0 {
		fmt.Fprintln(stdout, "\nSources:", strings.Join(result.SelectedIDs, ", "))
	}
	return nil
}

func runExport(config studio.RuntimeConfig, args []string, stdout io.Writer) error {
	projectName, remaining, err := leadingProject(args)
	if err != nil {
		return fmt.Errorf("usage: noligen export <project> --output <bundle.zip>")
	}
	flags := newFlagSet("export")
	output := flags.String("output", "", "ZIP output filename")
	if err := flags.Parse(remaining); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*output) == "" {
		return fmt.Errorf("usage: noligen export <project> --output <bundle.zip>")
	}
	workspace, err := project.OpenWorkspace(config.WorkspacePath, projectName)
	if err != nil {
		return err
	}
	problems := okf.ValidateBundle(workspace.KnowledgeDir, okf.StandardMode, okf.ValidationOptions{RequireBundleLog: true, RequireDirectoryIndexes: true})
	if err := okf.ProblemsError(problems); err != nil {
		return fmt.Errorf("export %s: %w", projectName, err)
	}
	if err := studio.ExportBundle(workspace.KnowledgeDir, *output); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Exported %s\n", *output)
	return nil
}

func configuredEngine(client llm.Client, config studio.RuntimeConfig) *studio.Engine {
	engine := studio.NewEngine(client)
	engine.MaximumChunk = config.MaximumChunk
	engine.RequestTimeout = config.RequestWait
	return engine
}

func ollamaClient(config studio.RuntimeConfig) (llm.Client, error) {
	client, err := llm.NewOllamaClient(llm.OllamaConfig{
		BaseURL: config.OllamaBaseURL, Model: config.OllamaModel, Timeout: config.RequestWait,
	})
	if err != nil {
		return nil, fmt.Errorf("configure Ollama: %w", err)
	}
	return client, nil
}

func openWorkspaceAndProfile(config studio.RuntimeConfig, name string) (project.Workspace, project.ProjectProfile, error) {
	workspace, err := project.OpenWorkspace(config.WorkspacePath, name)
	if err != nil {
		return project.Workspace{}, project.ProjectProfile{}, err
	}
	profile, err := studio.ActiveProfile(workspace)
	if err != nil {
		return project.Workspace{}, project.ProjectProfile{}, err
	}
	return workspace, profile, nil
}

func activeProfileIfAvailable(workspace project.Workspace) *project.ProjectProfile {
	profile, err := studio.ActiveProfile(workspace)
	if err != nil {
		return nil
	}
	return &profile
}

func printGenerationSummary(output io.Writer, workspace project.Workspace, result studio.GenerationResult) {
	fmt.Fprintf(output, "Generated %d canonical concepts from %d drafts and %d source documents\n", len(result.Concepts), len(result.Drafts), len(result.Documents))
	fmt.Fprintf(output, "Resolved %d relationships; %d unresolved\n", len(result.Edges), len(result.UnresolvedRelations))
	fmt.Fprintf(output, "Knowledge bundle: %s\n", workspace.KnowledgeDir)
}

func safeOutputDestination(value string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(value))
	if clean == "" || clean == "." || clean == ".." {
		return "", fmt.Errorf("one-command generation: output must name a dedicated knowledge directory")
	}
	absolute, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("one-command generation: resolve output %s: %w", value, err)
	}
	if filepath.Dir(absolute) == absolute {
		return "", fmt.Errorf("one-command generation: refusing filesystem-root output %s", absolute)
	}
	return absolute, nil
}

func leadingProject(args []string) (string, []string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", nil, fmt.Errorf("project name is required")
	}
	return args[0], args[1:], nil
}

func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

type stringListFlag []string

func (values *stringListFlag) String() string {
	return strings.Join(*values, ",")
}

func (values *stringListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("value must not be empty")
	}
	*values = append(*values, value)
	return nil
}

func warningCount(problems []okf.Problem) int {
	count := 0
	for _, problem := range problems {
		if problem.Severity == okf.SeverityWarning {
			count++
		}
	}
	return count
}

func firstCLIValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func writeUsage(output io.Writer) {
	commands := []string{
		"noligen init <project>",
		"noligen import <project> <path> [path ...]",
		"noligen profile <project> --goal <goal> --profile <auto|product-support|software-project|credit-card|file>",
		"noligen profile-show <project>",
		"noligen extract <project>",
		"noligen generate <project> [--strict]",
		"noligen generate --input <directory> --goal <goal> --output <directory> --profile <selection>",
		"noligen validate <project> --mode <standard|project>",
		"noligen list <project>",
		"noligen search <project> [--limit N] <query>",
		"noligen graph <project> <document-id>",
		"noligen ask <project> <question>",
		"noligen export <project> --output <bundle.zip>",
	}
	sort.Strings(commands)
	fmt.Fprintln(output, "Noli", okf.GeneratorVersion)
	fmt.Fprintln(output, "\nUsage:")
	for _, command := range commands {
		fmt.Fprintln(output, "  "+command)
	}
	fmt.Fprintln(output, "\nWorkspace root: $NOLI_WORKSPACE_PATH (default ./workspace)")
}
