package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/vovakirdan/durak/internal/adapters/ai"
)

const defaultRawAITimeout = 30 * time.Second

type rawAIFlags struct {
	command string
	args    repeatedStringFlag
	timeout time.Duration
}

type repeatedStringFlag []string

func newRawAIFlags() rawAIFlags {
	return rawAIFlags{timeout: defaultRawAITimeout}
}

func (f *rawAIFlags) bind(flags *flag.FlagSet) {
	flags.StringVar(&f.command, "ai-command", "", "external raw AI command for ai-raw-exec")
	flags.Var(&f.args, "ai-arg", "argument for ai-command; may be repeated")
	flags.DurationVar(&f.timeout, "ai-timeout", f.timeout, "timeout for one external raw AI turn")
}

func (f *rawAIFlags) client() (ai.Client, error) {
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
