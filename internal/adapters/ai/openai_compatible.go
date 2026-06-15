package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const defaultOpenAICompatibleTimeout = 30 * time.Second

var (
	// ErrMissingOpenAIModel means no chat model was configured.
	ErrMissingOpenAIModel = errors.New("missing openai-compatible model")
	// ErrMissingOpenAIAPIKey means the default OpenAI endpoint has no API key.
	ErrMissingOpenAIAPIKey = errors.New("missing openai-compatible api key")
	// ErrEmptyOpenAIResponse means the provider returned no text command.
	ErrEmptyOpenAIResponse = errors.New("empty openai-compatible response")
)

// OpenAICompatibleClientOptions configures an OpenAI-compatible chat endpoint.
type OpenAICompatibleClientOptions struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// OpenAICompatibleClient asks an OpenAI-compatible chat endpoint for a raw command.
type OpenAICompatibleClient struct {
	client  openai.Client
	model   string
	baseURL string
}

// NewOpenAICompatibleClient creates a raw-command client over /chat/completions.
func NewOpenAICompatibleClient(options OpenAICompatibleClientOptions) (*OpenAICompatibleClient, error) {
	if options.Model == "" {
		return nil, ErrMissingOpenAIModel
	}
	apiKey := options.APIKey
	if apiKey == "" {
		if options.BaseURL == "" {
			return nil, ErrMissingOpenAIAPIKey
		}
		// #nosec G101 -- dummy bearer value for local OpenAI-compatible endpoints that ignore auth.
		apiKey = "durak-local"
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultOpenAICompatibleTimeout
	}

	requestOptions := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(&http.Client{Timeout: timeout}),
	}
	if options.BaseURL != "" {
		requestOptions = append(requestOptions, option.WithBaseURL(options.BaseURL))
	}

	return &OpenAICompatibleClient{
		client:  openai.NewClient(requestOptions...),
		model:   options.Model,
		baseURL: options.BaseURL,
	}, nil
}

// ClientInfo returns non-secret provider metadata for diagnostics.
func (c *OpenAICompatibleClient) ClientInfo() ClientInfo {
	if c == nil {
		return ClientInfo{}
	}
	return ClientInfo{
		Provider: "openai-compatible",
		Model:    c.model,
		BaseURL:  safeProviderBaseURL(c.baseURL),
	}
}

// CompleteTurn runs one chat completion and returns the model's raw command.
func (c *OpenAICompatibleClient) CompleteTurn(
	ctx context.Context,
	prompt *TurnPrompt,
) (TurnResponse, error) {
	if c == nil || c.model == "" {
		return TurnResponse{}, ErrMissingOpenAIModel
	}

	userPrompt, err := marshalProviderPrompt(prompt)
	if err != nil {
		return TurnResponse{}, err
	}
	response, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(openAICommandSystemPrompt()),
			openai.UserMessage(userPrompt),
		},
		Model:       openai.ChatModel(c.model),
		Temperature: openai.Float(0),
		MaxTokens:   openai.Int(32),
	})
	if err != nil {
		return TurnResponse{}, fmt.Errorf("create openai-compatible chat completion: %w", err)
	}
	if len(response.Choices) == 0 {
		return TurnResponse{}, ErrEmptyOpenAIResponse
	}
	command := cleanProviderCommand(response.Choices[0].Message.Content)
	if command == "" {
		return TurnResponse{}, ErrEmptyOpenAIResponse
	}
	return TurnResponse{
		TextCommand: command,
		Usage: TokenUsage{
			PromptTokens:     response.Usage.PromptTokens,
			CompletionTokens: response.Usage.CompletionTokens,
			TotalTokens:      response.Usage.TotalTokens,
		},
	}, nil
}

func marshalProviderPrompt(prompt *TurnPrompt) (string, error) {
	payload, err := json.MarshalIndent(newSubprocessPrompt(prompt), "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal openai-compatible prompt: %w", err)
	}
	return string(payload), nil
}

func openAICommandSystemPrompt() string {
	return strings.Join([]string{
		"You are an AI player in a Russian Durak terminal game.",
		"Choose exactly one legal command from legal_actions[].command.",
		"Return only that command, without markdown or explanation.",
		"If previous_errors is present, avoid repeating the same invalid response.",
	}, " ")
}

func cleanProviderCommand(output string) string {
	for line := range strings.Lines(output) {
		command := strings.TrimSpace(strings.Trim(line, "`"))
		command = strings.TrimPrefix(command, "command:")
		command = strings.TrimPrefix(command, "Command:")
		command = strings.TrimPrefix(command, "answer:")
		command = strings.TrimPrefix(command, "Answer:")
		if command = strings.TrimSpace(command); command != "" && !strings.HasPrefix(command, "```") {
			return command
		}
	}
	return ""
}

func safeProviderBaseURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}
