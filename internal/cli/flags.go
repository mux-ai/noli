package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"noli/pkg/graph"
	"noli/pkg/protocol"
)

// flagError carries the offending flag for INVALID_ARGUMENT details.
type flagError struct {
	flag    string
	message string
}

func (e *flagError) Error() string { return e.message }

func (e *flagError) details() map[string]string {
	if e.flag == "" {
		return nil
	}
	return map[string]string{"flag": e.flag}
}

// newFlagSet builds a silent flag set; all errors surface as JSON.
func newFlagSet(command string) *flag.FlagSet {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	return fs
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	fs.String("format", "json", "output format (json)")
	if err := fs.Parse(args); err != nil {
		return &flagError{message: err.Error()}
	}
	if fs.NArg() > 0 {
		return &flagError{message: fmt.Sprintf("unexpected argument %q", fs.Arg(0))}
	}
	return nil
}

func splitTypes(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	types := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			types = append(types, trimmed)
		}
	}
	return types
}

func parseDirection(value string) (graph.Direction, error) {
	switch graph.Direction(value) {
	case graph.DirectionOutgoing, graph.DirectionIncoming, graph.DirectionBoth:
		return graph.Direction(value), nil
	default:
		return "", &flagError{
			flag:    "--direction",
			message: fmt.Sprintf("invalid direction %q; expected outgoing, incoming, or both", value),
		}
	}
}

func requireNonNegative(name string, value int) error {
	if value < 0 {
		return &flagError{flag: name, message: fmt.Sprintf("flag %s must be a non-negative integer", name)}
	}
	return nil
}

func requireValue(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return &flagError{flag: name, message: fmt.Sprintf("flag %s is required", name)}
	}
	return nil
}

// checkRoot validates the knowledge root before loading. It returns a stable
// error code and message when the root is unusable.
func checkRoot(root string) (code, message string, details map[string]string, ok bool) {
	if strings.TrimSpace(root) == "" {
		return protocol.CodeInvalidArgument, "flag --root is required",
			map[string]string{"flag": "--root"}, false
	}
	if strings.ContainsRune(root, '\x00') {
		return protocol.CodeUnsafePath, "knowledge root contains a NUL byte",
			map[string]string{"root": strings.ReplaceAll(root, "\x00", "")}, false
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return protocol.CodeKnowledgeNotFound,
			fmt.Sprintf("knowledge root %q does not exist or is not a directory", root),
			map[string]string{"root": root}, false
	}
	return "", "", nil, true
}
