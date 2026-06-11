package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/adapters/cli"
	"github.com/vovakirdan/durak/internal/domain"
)

func main() {
	var seed seedFlag
	flag.Var(&seed, "seed", "deterministic deal seed for replayable games")
	flag.Parse()

	options := cli.RunOptions{
		Strategy: bot.NewSimpleStrategy(),
	}
	if seed.set {
		options.Deal = domain.SeededDealOptions(seed.value)
	}

	if err := cli.RunWithOptions(context.Background(), os.Stdin, os.Stdout, &options); err != nil {
		fmt.Fprintf(os.Stderr, "durak: %v\n", err)
		os.Exit(1)
	}
}

type seedFlag struct {
	value uint64
	set   bool
}

func (s *seedFlag) Set(value string) error {
	seed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("parse seed: %w", err)
	}
	s.value = seed
	s.set = true
	return nil
}

func (s *seedFlag) String() string {
	if s == nil || !s.set {
		return ""
	}
	return strconv.FormatUint(s.value, 10)
}
