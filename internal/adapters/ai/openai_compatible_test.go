package ai_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vovakirdan/durak/internal/adapters/ai"
)

func TestOpenAICompatibleClientSendsChatCompletion(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotRequest chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("Decode request returned error: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"created":1,
			"model":"test-model",
			"choices":[
				{
					"index":0,
					"message":{"role":"assistant","content":"command: attack 6C"},
					"finish_reason":"stop"
				}
			],
			"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}
		}`))
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL + "/v1"
	client := mustOpenAICompatibleClient(t, ai.OpenAICompatibleClientOptions{
		BaseURL: baseURL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: time.Second,
	})

	response, err := client.CompleteTurn(t.Context(), subprocessTurnPrompt())
	if err != nil {
		t.Fatalf("CompleteTurn returned error: %v", err)
	}
	if response.TextCommand != "attack 6C" {
		t.Fatalf("TextCommand = %q, want attack command", response.TextCommand)
	}
	if response.Usage.TotalTokens != 12 {
		t.Fatalf("usage = %+v, want token usage", response.Usage)
	}
	if info := client.ClientInfo(); info.Provider != "openai-compatible" || info.BaseURL != baseURL {
		t.Fatalf("ClientInfo = %+v, want OpenAI-compatible metadata", info)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("path = %q, want chat completions path", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want bearer key", gotAuth)
	}
	if gotRequest.Model != "test-model" {
		t.Fatalf("model = %q, want test-model", gotRequest.Model)
	}
	if len(gotRequest.Messages) != 2 || gotRequest.Messages[1].Role != "user" {
		t.Fatalf("messages = %+v, want system and user messages", gotRequest.Messages)
	}
	if !strings.Contains(gotRequest.Messages[1].Content, `"command": "attack 6C"`) {
		t.Fatalf("user prompt = %q, want legal action command", gotRequest.Messages[1].Content)
	}
}

func TestOpenAICompatibleClientAllowsLocalEndpointWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"created":1,
			"model":"test-model",
			"choices":[{"index":0,"message":{"role":"assistant","content":"1"},"finish_reason":"stop"}]
		}`))
	}))
	t.Cleanup(server.Close)
	client := mustOpenAICompatibleClient(t, ai.OpenAICompatibleClientOptions{
		BaseURL: server.URL + "/v1",
		Model:   "test-model",
		Timeout: time.Second,
	})

	response, err := client.CompleteTurn(t.Context(), subprocessTurnPrompt())
	if err != nil {
		t.Fatalf("CompleteTurn returned error: %v", err)
	}
	if response.TextCommand != "1" {
		t.Fatalf("TextCommand = %q, want numbered command", response.TextCommand)
	}
}

func TestOpenAICompatibleClientRejectsEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"created":1,
			"model":"test-model",
			"choices":[]
		}`))
	}))
	t.Cleanup(server.Close)
	client := mustOpenAICompatibleClient(t, ai.OpenAICompatibleClientOptions{
		BaseURL: server.URL + "/v1",
		Model:   "test-model",
		Timeout: time.Second,
	})

	_, err := client.CompleteTurn(t.Context(), subprocessTurnPrompt())
	if !errors.Is(err, ai.ErrEmptyOpenAIResponse) {
		t.Fatalf("error = %v, want ErrEmptyOpenAIResponse", err)
	}
}

func TestNewOpenAICompatibleClientRequiresModel(t *testing.T) {
	_, err := ai.NewOpenAICompatibleClient(ai.OpenAICompatibleClientOptions{
		BaseURL: "http://127.0.0.1:11434/v1",
	})
	if !errors.Is(err, ai.ErrMissingOpenAIModel) {
		t.Fatalf("error = %v, want ErrMissingOpenAIModel", err)
	}
}

func TestNewOpenAICompatibleClientRequiresAPIKeyForDefaultEndpoint(t *testing.T) {
	_, err := ai.NewOpenAICompatibleClient(ai.OpenAICompatibleClientOptions{Model: "test-model"})
	if !errors.Is(err, ai.ErrMissingOpenAIAPIKey) {
		t.Fatalf("error = %v, want ErrMissingOpenAIAPIKey", err)
	}
}

func TestOpenAICompatibleClientInfoRedactsBaseURLCredentials(t *testing.T) {
	client := mustOpenAICompatibleClient(t, ai.OpenAICompatibleClientOptions{
		BaseURL: "http://user:secret@example.test/v1?api_key=secret#fragment",
		Model:   "test-model",
		Timeout: time.Second,
	})

	if info := client.ClientInfo(); info.BaseURL != "http://example.test/v1" {
		t.Fatalf("BaseURL = %q, want redacted URL", info.BaseURL)
	}
}

type chatCompletionRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func mustOpenAICompatibleClient(
	t *testing.T,
	options ai.OpenAICompatibleClientOptions,
) *ai.OpenAICompatibleClient {
	t.Helper()
	client, err := ai.NewOpenAICompatibleClient(options)
	if err != nil {
		t.Fatalf("NewOpenAICompatibleClient returned error: %v", err)
	}
	return client
}
