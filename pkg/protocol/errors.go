package protocol

// Stable error codes (docs/PROTOCOL.md section 3).
const (
	CodeInvalidArgument      = "INVALID_ARGUMENT"
	CodeKnowledgeNotFound    = "KNOWLEDGE_NOT_FOUND"
	CodeDocumentNotFound     = "DOCUMENT_NOT_FOUND"
	CodeUnsafePath           = "UNSAFE_PATH"
	CodeParseError           = "PARSE_ERROR"
	CodeInvalidFrontmatter   = "INVALID_FRONTMATTER"
	CodeValidationFailed     = "VALIDATION_FAILED"
	CodeGenerationFailed     = "GENERATION_FAILED"
	CodeContextLimitTooSmall = "CONTEXT_LIMIT_TOO_SMALL"
	CodeInternalError        = "INTERNAL_ERROR"
)

// Exit codes (docs/PROTOCOL.md section 2).
const (
	ExitSuccess         = 0
	ExitInvalidArgument = 2
	ExitLoadFailure     = 3
	ExitValidation      = 4
	ExitGeneration      = 5
	ExitUnsafePath      = 6
	ExitInternal        = 7
)

// ExitCodeFor maps a stable error code to its frozen exit code. Unknown
// codes map to ExitInternal.
func ExitCodeFor(code string) int {
	switch code {
	case CodeInvalidArgument, CodeContextLimitTooSmall:
		return ExitInvalidArgument
	case CodeKnowledgeNotFound, CodeDocumentNotFound, CodeParseError, CodeInvalidFrontmatter:
		return ExitLoadFailure
	case CodeValidationFailed:
		return ExitValidation
	case CodeGenerationFailed:
		return ExitGeneration
	case CodeUnsafePath:
		return ExitUnsafePath
	default:
		return ExitInternal
	}
}
