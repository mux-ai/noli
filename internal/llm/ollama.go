package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL      = "http://localhost:11434"
	DefaultModel        = "gemma3"
	defaultErrorBodyMax = 4096
)

type OllamaConfig struct {
	BaseURL    string
	Model      string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type OllamaClient struct {
	baseURL string
	model   string
	timeout time.Duration
	http    *http.Client
}

func NewOllamaClient(config OllamaConfig) (*OllamaClient, error) {
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("create Ollama client: parse base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("create Ollama client: base URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("create Ollama client: base URL has no host")
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = DefaultModel
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &OllamaClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		timeout: timeout,
		http:    httpClient,
	}, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Format   string        `json:"format,omitempty"`
}

type chatResponse struct {
	Message chatMessage `json:"message"`
	Done    bool        `json:"done"`
	Error   string      `json:"error,omitempty"`
}

func (c *OllamaClient) GenerateStructured(ctx context.Context, systemPrompt, userPrompt string, output any) error {
	content, err := c.generate(ctx, systemPrompt, userPrompt, true)
	if err != nil {
		return fmt.Errorf("generate structured response: %w", err)
	}
	if err := DecodeStrictJSON([]byte(content), output); err != nil {
		return fmt.Errorf("generate structured response: %w", err)
	}
	return nil
}

func (c *OllamaClient) Chat(ctx context.Context, systemPrompt, question, contextText string) (string, error) {
	var userPrompt strings.Builder
	if strings.TrimSpace(contextText) != "" {
		userPrompt.WriteString("KNOWLEDGE CONTEXT:\n")
		userPrompt.WriteString(contextText)
		userPrompt.WriteString("\n\n")
	}
	userPrompt.WriteString("QUESTION:\n")
	userPrompt.WriteString(question)
	content, err := c.generate(ctx, systemPrompt, userPrompt.String(), false)
	if err != nil {
		return "", fmt.Errorf("chat: %w", err)
	}
	return content, nil
}

func (c *OllamaClient) generate(ctx context.Context, systemPrompt, userPrompt string, structured bool) (string, error) {
	if c == nil {
		return "", fmt.Errorf("call Ollama: client is nil")
	}
	messages := make([]chatMessage, 0, 2)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, chatMessage{Role: "user", Content: userPrompt})
	requestBody := chatRequest{Model: c.model, Messages: messages, Stream: false}
	if structured {
		requestBody.Format = "json"
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		excerpt, readErr := io.ReadAll(io.LimitReader(response.Body, defaultErrorBodyMax+1))
		if readErr != nil {
			return "", fmt.Errorf("Ollama returned status %d; read error response: %w", response.StatusCode, readErr)
		}
		if len(excerpt) > defaultErrorBodyMax {
			excerpt = append(excerpt[:defaultErrorBodyMax], []byte("...")...)
		}
		return "", fmt.Errorf("Ollama returned status %d: %s", response.StatusCode, strings.TrimSpace(string(excerpt)))
	}

	limited := io.LimitReader(response.Body, 32<<20)
	var decoded chatResponse
	decoder := json.NewDecoder(limited)
	if err := decoder.Decode(&decoded); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return "", fmt.Errorf("decode response: multiple JSON values")
		}
		return "", fmt.Errorf("decode response trailing data: %w", err)
	}
	if strings.TrimSpace(decoded.Error) != "" {
		return "", fmt.Errorf("Ollama response error: %s", decoded.Error)
	}
	if decoded.Message.Role != "" && decoded.Message.Role != "assistant" {
		return "", fmt.Errorf("decode response: unexpected message role %q", decoded.Message.Role)
	}
	if strings.TrimSpace(decoded.Message.Content) == "" {
		return "", fmt.Errorf("decode response: empty assistant content")
	}
	return decoded.Message.Content, nil
}
