package cli

import (
	"errors"
	"fmt"

	"noli/pkg/generator"
	"noli/pkg/protocol"
)

// runGenerate implements "generate --config noli.yaml (--dry-run | --apply)".
// Exactly one of --dry-run and --apply is required. Active knowledge is
// never modified without --apply, and a failed apply rolls back.
func (a *App) runGenerate(args []string) int {
	fs := newFlagSet("generate")
	configPath := fs.String("config", "", "noli.yaml path")
	dryRun := fs.Bool("dry-run", false, "render into the preview directory only")
	apply := fs.Bool("apply", false, "validate and replace the active knowledge root")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("generate", err)
	}
	if err := requireValue("--config", *configPath); err != nil {
		return a.invalidArgument("generate", err)
	}
	if *dryRun == *apply {
		return a.failure("generate", protocol.CodeInvalidArgument,
			"exactly one of --dry-run and --apply is required",
			map[string]string{"flag": "--dry-run"})
	}
	config, err := generator.LoadConfig(*configPath)
	if err != nil {
		return a.configFailure("generate", *configPath, err)
	}
	result, err := generator.Generate(config, generator.GenerateOptions{Apply: *apply})
	if err != nil {
		var validation *generator.BundleValidationError
		switch {
		case errors.As(err, &validation):
			return a.failure("generate", protocol.CodeValidationFailed,
				"generated bundle failed validation; active knowledge was left unchanged",
				map[string]string{"errors": fmt.Sprint(len(validation.Errors))})
		case errors.Is(err, generator.ErrUnsafePath):
			return a.failure("generate", protocol.CodeUnsafePath, err.Error(),
				map[string]string{"config": *configPath})
		default:
			return a.failure("generate", protocol.CodeGenerationFailed, err.Error(),
				map[string]string{"config": *configPath})
		}
	}
	return a.success("generate", protocol.GenerateData{
		Mode:        result.Mode,
		PreviewRoot: result.PreviewRoot,
		Added:       result.Added,
		Changed:     result.Changed,
		Removed:     result.Removed,
		Unchanged:   result.Unchanged,
	})
}
