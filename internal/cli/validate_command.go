package cli

import (
	"errors"
	"fmt"

	"noli/pkg/generator"
	"noli/pkg/okf"
	"noli/pkg/protocol"
)

// runValidate implements "validate --mode standard|project". Project mode
// requires --config; the knowledge root comes from --root or, when absent,
// from the configuration. An invalid bundle stays a success envelope and
// carries exit code 4 (docs/PROTOCOL.md section 3).
func (a *App) runValidate(args []string) int {
	fs := newFlagSet("validate")
	root := fs.String("root", "", "knowledge root directory")
	mode := fs.String("mode", "standard", "validation mode (standard, project)")
	configPath := fs.String("config", "", "noli.yaml path (project mode)")
	if err := parseFlags(fs, args); err != nil {
		return a.invalidArgument("validate", err)
	}

	options := okf.ValidationOptions{}
	switch *mode {
	case "standard":
	case "project":
		if *configPath == "" {
			return a.failure("validate", protocol.CodeInvalidArgument,
				"project mode requires --config", map[string]string{"flag": "--config"})
		}
		config, err := generator.LoadConfig(*configPath)
		if err != nil {
			return a.configFailure("validate", *configPath, err)
		}
		options = config.ValidationOptions()
		if *root == "" {
			resolved, err := config.KnowledgeRoot()
			if err != nil {
				return a.configFailure("validate", *configPath, err)
			}
			*root = resolved
		}
	default:
		return a.failure("validate", protocol.CodeInvalidArgument,
			fmt.Sprintf("unsupported validation mode %q; expected standard or project", *mode),
			map[string]string{"flag": "--mode"})
	}

	if code, message, details, ok := checkRoot(*root); !ok {
		return a.failure("validate", code, message, details)
	}
	report := okf.Validate(*root, options)
	data := protocol.ValidateData{
		Mode:     *mode,
		Valid:    report.Valid,
		Errors:   toProblemData(report.Errors),
		Warnings: toProblemData(report.Warnings),
	}
	exit := a.success("validate", data)
	if exit == protocol.ExitSuccess && !report.Valid {
		return protocol.ExitValidation
	}
	return exit
}

// configFailure maps configuration errors onto the frozen protocol codes.
func (a *App) configFailure(command, path string, err error) int {
	details := map[string]string{"config": path}
	switch {
	case errors.Is(err, generator.ErrNotFound):
		return a.failure(command, protocol.CodeKnowledgeNotFound, err.Error(), details)
	case errors.Is(err, generator.ErrUnsafePath):
		return a.failure(command, protocol.CodeUnsafePath, err.Error(), details)
	default:
		return a.failure(command, protocol.CodeParseError, err.Error(), details)
	}
}

func toProblemData(problems []okf.Problem) []protocol.ValidationProblemData {
	result := make([]protocol.ValidationProblemData, 0, len(problems))
	for _, problem := range problems {
		result = append(result, protocol.ValidationProblemData{
			Code:     problem.Code,
			Document: problem.Document,
			Message:  problem.Message,
		})
	}
	return result
}
