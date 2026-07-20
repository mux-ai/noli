// Package cli implements the Noli command-line interface on top of the public
// SDK and the frozen JSON protocol. Handlers return exit codes; only command
// entry points call os.Exit.
package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"noli/pkg/protocol"
)

// App is one CLI invocation environment with injectable streams.
type App struct {
	stdout   io.Writer
	stderr   io.Writer
	commands map[string]func(args []string) int
	command  string
}

// New builds an App with the built-in read-only commands.
func New(stdout, stderr io.Writer) *App {
	app := &App{stdout: stdout, stderr: stderr}
	app.commands = map[string]func(args []string) int{
		"status":                app.runStatus,
		"list":                  app.runList,
		"search":                app.runSearch,
		"retrieve":              app.runRetrieve,
		"get":                   app.runGet,
		"graph":                 app.runGraph,
		"validate":              app.runValidate,
		"generate":              app.runGenerate,
		"prepare-agent-context": app.runPrepare,
	}
	return app
}

// Run dispatches one command line (without the program name) and returns the
// process exit code.
func (a *App) Run(args []string) (exit int) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(a.stderr, "panic: %v\n", recovered)
			exit = a.failure(a.command, protocol.CodeInternalError,
				"an unexpected internal error occurred", nil)
		}
	}()
	if len(args) == 0 {
		return a.failure("", protocol.CodeInvalidArgument,
			"missing command; expected one of: "+strings.Join(a.commandNames(), ", "), nil)
	}
	name := args[0]
	if err := scanFormat(args); err != nil {
		command := ""
		if _, known := a.commands[name]; known {
			command = name
		}
		return a.failure(command, protocol.CodeInvalidArgument, err.Error(),
			map[string]string{"flag": "--format"})
	}
	handler, known := a.commands[name]
	if !known {
		return a.failure("", protocol.CodeInvalidArgument,
			fmt.Sprintf("unknown command %q; expected one of: %s", name, strings.Join(a.commandNames(), ", ")), nil)
	}
	a.command = name
	return handler(args[1:])
}

func (a *App) commandNames() []string {
	names := make([]string, 0, len(a.commands))
	for name := range a.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// scanFormat runs before normal flag parsing so malformed JSON-mode
// invocations still receive a JSON error. Only "json" is supported.
func scanFormat(args []string) error {
	for i := range len(args) {
		value := ""
		switch {
		case args[i] == "--format" || args[i] == "-format":
			if i+1 >= len(args) {
				return fmt.Errorf("flag --format requires a value")
			}
			value = args[i+1]
		case strings.HasPrefix(args[i], "--format="):
			value = strings.TrimPrefix(args[i], "--format=")
		case strings.HasPrefix(args[i], "-format="):
			value = strings.TrimPrefix(args[i], "-format=")
		default:
			continue
		}
		if value != "json" {
			return fmt.Errorf("unsupported format %q; only \"json\" is available", value)
		}
	}
	return nil
}

func (a *App) success(command string, data any) int {
	if err := protocol.Write(a.stdout, protocol.Success(command, data)); err != nil {
		fmt.Fprintf(a.stderr, "write response: %v\n", err)
		return protocol.ExitInternal
	}
	return protocol.ExitSuccess
}

func (a *App) failure(command, code, message string, details map[string]string) int {
	if err := protocol.Write(a.stdout, protocol.Failure(command, code, message, details)); err != nil {
		fmt.Fprintf(a.stderr, "write response: %v\n", err)
		return protocol.ExitInternal
	}
	return protocol.ExitCodeFor(code)
}
