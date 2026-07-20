package llm

import "context"

// Client is the small language-model boundary used by profiling, extraction,
// and retrieval. Implementations must decode structured output into output.
type Client interface {
	GenerateStructured(
		ctx context.Context,
		systemPrompt string,
		userPrompt string,
		output any,
	) error

	Chat(
		ctx context.Context,
		systemPrompt string,
		question string,
		contextText string,
	) (string, error)
}
