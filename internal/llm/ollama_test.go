package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOllamaGenerateStructuredAndChat(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.URL.Path != "/api/chat" {
			t.Errorf("path = %q", request.URL.Path)
		}
		var body chatRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		if body.Format == "json" {
			_, _ = writer.Write([]byte(`{"message":{"role":"assistant","content":"{\"name\":\"structured\"}"},"done":true}`))
			return
		}
		if !strings.Contains(body.Messages[len(body.Messages)-1].Content, "KNOWLEDGE CONTEXT:") {
			t.Error("chat request omitted context label")
		}
		_, _ = writer.Write([]byte(`{"message":{"role":"assistant","content":"answer"},"done":true}`))
	}))
	defer server.Close()
	client, err := NewOllamaClient(OllamaConfig{BaseURL: server.URL, Model: "test", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Name string `json:"name"`
	}
	if err := client.GenerateStructured(context.Background(), "system", "user", &output); err != nil {
		t.Fatal(err)
	}
	if output.Name != "structured" {
		t.Fatalf("name = %q", output.Name)
	}
	answer, err := client.Chat(context.Background(), "system", "question", "context")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "answer" || requests != 2 {
		t.Fatalf("answer = %q, requests = %d", answer, requests)
	}
}

func TestOllamaNon2xxIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte(strings.Repeat("x", defaultErrorBodyMax+100)))
	}))
	defer server.Close()
	client, err := NewOllamaClient(OllamaConfig{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Chat(context.Background(), "", "question", "")
	if err == nil {
		t.Fatal("expected non-2xx error")
	}
	if !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("error = %v", err)
	}
	if len(err.Error()) > defaultErrorBodyMax+200 {
		t.Fatalf("error body was not bounded: %d bytes", len(err.Error()))
	}
}
