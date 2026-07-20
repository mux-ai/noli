package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"noli/pkg/okf"
	"noli/pkg/protocol"
	"noli/pkg/retrieval"
)

// runPrepare implements "prepare-agent-context --root <knowledge> --config
// <noli-agent-queries.yaml> --output <directory>". The output is built in a
// temporary sibling and replaced atomically; an existing output is replaced
// only when it is empty or a previous prepared context.
func (a *App) runPrepare(args []string) int {
	fs := newFlagSet("prepare-agent-context")
	root := fs.String("root", "", "knowledge root directory")
	configPath := fs.String("config", "", "noli-agent-queries.yaml path")
	output := fs.String("output", "", "output directory for prepared contexts")
	timestamp := fs.String("timestamp", "", "manifest timestamp (RFC3339; default now UTC)")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("prepare-agent-context", err)
	}
	if err := requireValue("--config", *configPath); err != nil {
		return a.invalidArgument("prepare-agent-context", err)
	}
	if err := requireValue("--output", *output); err != nil {
		return a.invalidArgument("prepare-agent-context", err)
	}
	generatedAt := time.Now().UTC()
	if *timestamp != "" {
		parsed, err := time.Parse(time.RFC3339, *timestamp)
		if err != nil {
			return a.failure("prepare-agent-context", protocol.CodeInvalidArgument,
				fmt.Sprintf("invalid --timestamp %q; expected RFC3339", *timestamp),
				map[string]string{"flag": "--timestamp"})
		}
		generatedAt = parsed
	}
	if code, message, details, ok := checkRoot(*root); !ok {
		return a.failure("prepare-agent-context", code, message, details)
	}
	if code, message, ok := checkOutputPath(*output, *root); !ok {
		return a.failure("prepare-agent-context", code, message,
			map[string]string{"path": *output})
	}
	data, err := os.ReadFile(*configPath)
	if err != nil {
		return a.failure("prepare-agent-context", protocol.CodeKnowledgeNotFound,
			fmt.Sprintf("agent queries file %q does not exist", *configPath),
			map[string]string{"config": *configPath})
	}
	queries, err := retrieval.ParseQueries(data)
	if err != nil {
		return a.failure("prepare-agent-context", protocol.CodeParseError, err.Error(),
			map[string]string{"config": *configPath})
	}
	store, exit, ok := a.loadStore("prepare-agent-context", *root)
	if !ok {
		return exit
	}
	retrieve := func(query string, options retrieval.Options) (retrieval.Result, error) {
		return store.Retrieve(query, okf.RetrieveOptions{
			SearchLimit:   options.SearchLimit,
			MaxHops:       options.MaxHops,
			MaxDocuments:  options.MaxDocuments,
			MaxCharacters: options.MaxCharacters,
			Direction:     options.Direction,
			IncludeTypes:  options.IncludeTypes,
			ExcludeTypes:  options.ExcludeTypes,
		})
	}
	prepared, err := retrieval.Prepare(queries, retrieve, retrieval.PrepareOptions{
		Output:      *output,
		BundleID:    store.BundleID(),
		GeneratedAt: generatedAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, retrieval.ErrWriteBusy):
			return a.failure("prepare-agent-context", protocol.CodeGenerationFailed,
				err.Error(), map[string]string{"path": *output})
		case errors.Is(err, retrieval.ErrContextLimitTooSmall):
			return a.failure("prepare-agent-context", protocol.CodeContextLimitTooSmall,
				err.Error(), nil)
		case errors.Is(err, retrieval.ErrUnsafeOutput):
			return a.failure("prepare-agent-context", protocol.CodeUnsafePath, err.Error(),
				map[string]string{"path": *output})
		default:
			return a.failure("prepare-agent-context", protocol.CodeInternalError, err.Error(), nil)
		}
	}
	queriesData := make([]protocol.PrepareQueryData, 0, len(prepared.Queries))
	for _, query := range prepared.Queries {
		queriesData = append(queriesData, protocol.PrepareQueryData{
			Name:      query.Name,
			File:      query.File,
			Checksum:  query.Checksum,
			Sources:   query.Sources,
			Truncated: query.Truncated,
		})
	}
	return a.success("prepare-agent-context", protocol.PrepareData{
		Output:      prepared.Output,
		BundleID:    prepared.BundleID,
		GeneratedAt: prepared.GeneratedAt,
		Manifest:    prepared.Manifest,
		Queries:     queriesData,
	})
}

// checkOutputPath refuses NUL bytes and any overlap between the output
// directory and the knowledge root (after resolving to absolute paths).
func checkOutputPath(output, root string) (code, message string, ok bool) {
	if strings.ContainsRune(output, '\x00') {
		return protocol.CodeUnsafePath, "output path contains a NUL byte", false
	}
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return protocol.CodeUnsafePath, fmt.Sprintf("resolve output path %q: %v", output, err), false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return protocol.CodeUnsafePath, fmt.Sprintf("resolve knowledge root %q: %v", root, err), false
	}
	if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = resolved
	}
	if resolvedOutput, err := filepath.EvalSymlinks(absOutput); err == nil {
		absOutput = resolvedOutput
	}
	separator := string(filepath.Separator)
	if absOutput == absRoot ||
		strings.HasPrefix(absOutput+separator, absRoot+separator) ||
		strings.HasPrefix(absRoot+separator, absOutput+separator) {
		return protocol.CodeUnsafePath,
			"output path overlaps the knowledge root after symlink resolution", false
	}
	return "", "", true
}
