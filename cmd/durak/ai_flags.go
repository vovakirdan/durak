package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vovakirdan/durak/internal/adapters/ai"
)

const defaultRawAITimeout = 30 * time.Second

type aiFlags struct {
	command string
	args    repeatedStringFlag
	timeout time.Duration
	baseURL string
	apiKey  string
	model   string
}

type repeatedStringFlag []string

func newAIFlags() aiFlags {
	return aiFlags{
		timeout: defaultRawAITimeout,
		baseURL: firstEnv(
			"DURAK_AI_BASE_URL",
			"OPENAI_BASE_URL",
		),
		apiKey: firstEnv(
			"DURAK_AI_API_KEY",
			"OPENAI_API_KEY",
		),
		model: firstEnv(
			"DURAK_AI_MODEL",
			"OPENAI_MODEL",
		),
	}
}

func (f *aiFlags) bind(flags *flag.FlagSet) {
	flags.StringVar(&f.command, "ai-command", "", "external raw AI command for ai-raw-exec")
	flags.Var(&f.args, "ai-arg", "argument for ai-command; may be repeated")
	flags.DurationVar(&f.timeout, "ai-timeout", f.timeout, "timeout for one external raw AI turn")
	flags.StringVar(&f.baseURL, "ai-base-url", f.baseURL, "OpenAI-compatible API base URL for ai-openai")
	flags.StringVar(&f.apiKey, "ai-api-key", f.apiKey, "OpenAI-compatible API key for ai-openai; prefer env in shared shells")
	flags.StringVar(&f.model, "ai-model", f.model, "OpenAI-compatible chat model for ai-openai")
}

func (f *aiFlags) subprocessClient() (ai.Client, error) {
	if f.command == "" {
		return nil, nil
	}
	client, err := ai.NewSubprocessClient(ai.SubprocessClientOptions{
		Command: f.command,
		Args:    []string(f.args),
		Timeout: f.timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("create raw ai client: %w", err)
	}
	return client, nil
}

func (f *aiFlags) openAICompatibleClient() (ai.Client, error) {
	client, err := ai.NewOpenAICompatibleClient(ai.OpenAICompatibleClientOptions{
		BaseURL: f.baseURL,
		APIKey:  f.apiKey,
		Model:   f.model,
		Timeout: f.timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("create openai-compatible ai client: %w", err)
	}
	return client, nil
}

func (f *aiFlags) clientForKind(kind string) (ai.Client, error) {
	switch normalizePlayerControllerKind(kind) {
	case controllerAIRawExec:
		return f.subprocessClient()
	case controllerAIOpenAI:
		return f.openAICompatibleClient()
	default:
		return nil, nil
	}
}

func (f *repeatedStringFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func (f *repeatedStringFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(*f, " ")
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
