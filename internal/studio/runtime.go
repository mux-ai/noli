package studio

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"noli/internal/llm"
)

const (
	DefaultWorkspacePath = "workspace"
	DefaultContextLimit  = 24000
	DefaultChunkLimit    = 12000
	DefaultRequestWait   = 2 * time.Minute
)

type RuntimeConfig struct {
	WorkspacePath string
	OllamaBaseURL string
	OllamaModel   string
	MaximumChunk  int
	ContextLimit  int
	RequestWait   time.Duration
}

// RuntimeConfigFromEnv parses all supported environment variables and rejects
// invalid limits instead of silently falling back.
func RuntimeConfigFromEnv() (RuntimeConfig, error) {
	config := RuntimeConfig{
		WorkspacePath: envOrFallback("NOLI_WORKSPACE_PATH", "OKF_WORKSPACE_PATH", DefaultWorkspacePath),
		OllamaBaseURL: envOrDefault("OLLAMA_BASE_URL", llm.DefaultBaseURL),
		OllamaModel:   envOrDefault("OLLAMA_MODEL", llm.DefaultModel),
		MaximumChunk:  DefaultChunkLimit,
		ContextLimit:  DefaultContextLimit,
		RequestWait:   DefaultRequestWait,
	}
	var err error
	if name, value := firstEnv("NOLI_MAX_CHUNK_CHARS", "OKF_MAX_CHUNK_CHARS"); value != "" {
		config.MaximumChunk, err = positiveInteger(name, value)
		if err != nil {
			return RuntimeConfig{}, err
		}
	}
	if name, value := firstEnv("NOLI_CONTEXT_LIMIT", "OKF_CONTEXT_LIMIT"); value != "" {
		config.ContextLimit, err = positiveInteger(name, value)
		if err != nil {
			return RuntimeConfig{}, err
		}
	}
	if name, value := firstEnv("NOLI_REQUEST_TIMEOUT", "OKF_REQUEST_TIMEOUT"); value != "" {
		config.RequestWait, err = time.ParseDuration(value)
		if err != nil || config.RequestWait <= 0 {
			if err == nil {
				err = fmt.Errorf("duration must be positive")
			}
			return RuntimeConfig{}, fmt.Errorf("parse %s %q: %w", name, value, err)
		}
	}
	return config, nil
}

func envOrFallback(primary, legacy, fallback string) string {
	if _, value := firstEnv(primary, legacy); value != "" {
		return value
	}
	return fallback
}

func firstEnv(primary, legacy string) (string, string) {
	if value := strings.TrimSpace(os.Getenv(primary)); value != "" {
		return primary, value
	}
	if value := strings.TrimSpace(os.Getenv(legacy)); value != "" {
		return legacy, value
	}
	return primary, ""
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func positiveInteger(name, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s %q: %w", name, value, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("parse %s %q: value must be positive", name, value)
	}
	return parsed, nil
}
