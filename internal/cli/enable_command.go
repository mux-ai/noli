package cli

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"noli/pkg/generator"
	"noli/pkg/protocol"
)

// Embedded copies of the shared starter templates so "enable" can bootstrap
// a knowledge base without any installed integration. A test asserts they
// stay byte-identical to integrations/shared.
//
//go:embed starter/noli-starter.yaml starter/noli-starter-concepts.yaml
var starterTemplates embed.FS

var starterNameLine = regexp.MustCompile(`(?m)^  name: .*$`)

// runEnable implements "enable --dir <repository>". It removes opt-out
// sentinels and, when no knowledge base exists yet, bootstraps the starter
// configuration and concepts, then generates and validates the bundle. The
// resulting file state enables Noli for every agent at once.
func (a *App) runEnable(args []string) int {
	fs := newFlagSet("enable")
	dir := fs.String("dir", ".", "repository directory")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("enable", err)
	}
	directory, code := a.checkStateDir("enable", *dir)
	if code != protocol.ExitSuccess {
		return code
	}

	data := protocol.EnableData{
		Dir:              *dir,
		State:            "enabled",
		Created:          []string{},
		RemovedSentinels: []string{},
	}
	for _, sentinel := range []string{
		filepath.Join(".noli", "disabled"),
		filepath.Join(".okf", "disabled"),
	} {
		path := filepath.Join(directory, sentinel)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := os.Remove(path); err != nil {
			return a.failure("enable", protocol.CodeInternalError,
				fmt.Sprintf("remove opt-out sentinel %q: %v", sentinel, err), nil)
		}
		data.RemovedSentinels = append(data.RemovedSentinels, filepath.ToSlash(sentinel))
	}

	configPath := filepath.Join(directory, "noli.yaml")
	hasConfig := fileExists(configPath)
	hasKnowledge := dirExists(filepath.Join(directory, "knowledge"))
	if !hasConfig && !hasKnowledge {
		template, err := starterTemplates.ReadFile("starter/noli-starter.yaml")
		if err != nil {
			return a.failure("enable", protocol.CodeInternalError, err.Error(), nil)
		}
		name := projectNameFor(directory)
		content := starterNameLine.ReplaceAllString(string(template), "  name: "+name)
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			return a.failure("enable", protocol.CodeInternalError,
				fmt.Sprintf("write noli.yaml: %v", err), nil)
		}
		data.Created = append(data.Created, "noli.yaml")
		hasConfig = true

		conceptsPath := filepath.Join(directory, ".noli", "concepts.yaml")
		if !fileExists(conceptsPath) {
			concepts, err := starterTemplates.ReadFile("starter/noli-starter-concepts.yaml")
			if err != nil {
				return a.failure("enable", protocol.CodeInternalError, err.Error(), nil)
			}
			if err := os.MkdirAll(filepath.Dir(conceptsPath), 0o755); err != nil {
				return a.failure("enable", protocol.CodeInternalError,
					fmt.Sprintf("create .noli: %v", err), nil)
			}
			if err := os.WriteFile(conceptsPath, concepts, 0o644); err != nil {
				return a.failure("enable", protocol.CodeInternalError,
					fmt.Sprintf("write concepts: %v", err), nil)
			}
			data.Created = append(data.Created, ".noli/concepts.yaml")
		}
	}

	if hasConfig && !dirExists(filepath.Join(directory, "knowledge")) {
		config, err := generator.LoadConfig(configPath)
		if err != nil {
			return a.configFailure("enable", configPath, err)
		}
		if _, err := generator.Generate(config, generator.GenerateOptions{Apply: true}); err != nil {
			var validation *generator.BundleValidationError
			switch {
			case errors.As(err, &validation):
				return a.failure("enable", protocol.CodeValidationFailed,
					"generated bundle failed validation; active knowledge was left unchanged",
					map[string]string{"errors": fmt.Sprint(len(validation.Errors))})
			case errors.Is(err, generator.ErrUnsafePath):
				return a.failure("enable", protocol.CodeUnsafePath, err.Error(),
					map[string]string{"config": configPath})
			default:
				return a.failure("enable", protocol.CodeGenerationFailed, err.Error(),
					map[string]string{"config": configPath})
			}
		}
		data.Generated = true
	}

	data.Changed = len(data.Created) > 0 || len(data.RemovedSentinels) > 0 || data.Generated
	return a.success("enable", data)
}

// runDisable implements "disable --dir <repository>": it records the
// developer's opt-out so no agent asks or retrieves in this repository.
func (a *App) runDisable(args []string) int {
	fs := newFlagSet("disable")
	dir := fs.String("dir", ".", "repository directory")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("disable", err)
	}
	directory, code := a.checkStateDir("disable", *dir)
	if code != protocol.ExitSuccess {
		return code
	}
	sentinel := filepath.Join(directory, ".noli", "disabled")
	changed := !fileExists(sentinel)
	if changed {
		if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
			return a.failure("disable", protocol.CodeInternalError,
				fmt.Sprintf("create .noli: %v", err), nil)
		}
		if err := os.WriteFile(sentinel, []byte("developer opted out\n"), 0o644); err != nil {
			return a.failure("disable", protocol.CodeInternalError,
				fmt.Sprintf("write sentinel: %v", err), nil)
		}
	}
	return a.success("disable", protocol.DisableData{
		Dir:      *dir,
		State:    "disabled",
		Changed:  changed,
		Sentinel: ".noli/disabled",
	})
}

// checkStateDir validates the repository directory shared by enable and
// disable. On failure the error envelope has already been written.
func (a *App) checkStateDir(command, dir string) (string, int) {
	if strings.TrimSpace(dir) == "" {
		return "", a.failure(command, protocol.CodeInvalidArgument,
			"flag --dir is required", map[string]string{"flag": "--dir"})
	}
	if strings.ContainsRune(dir, '\x00') {
		return "", a.failure(command, protocol.CodeUnsafePath,
			"repository directory contains a NUL byte", nil)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", a.failure(command, protocol.CodeKnowledgeNotFound,
			fmt.Sprintf("repository directory %q does not exist or is not a directory", dir),
			map[string]string{"dir": dir})
	}
	return dir, protocol.ExitSuccess
}

// projectNameFor derives a quoted YAML project name from the directory.
func projectNameFor(directory string) string {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		absolute = directory
	}
	name := filepath.Base(absolute)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "My Project"
	}
	name = strings.ReplaceAll(name, `\`, `\\`)
	name = strings.ReplaceAll(name, `"`, `\"`)
	return `"` + name + `"`
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
