package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/adapters/cli"
	"github.com/vovakirdan/durak/internal/adapters/storage"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func main() {
	var seed seedFlag
	var eventLogPath string
	var matchID string
	flag.Var(&seed, "seed", "deterministic deal seed for replayable games")
	flag.StringVar(&eventLogPath, "event-log", "", "append public match events to a JSONL file")
	flag.StringVar(&matchID, "match-id", "", "match id for event log; generated when omitted")
	flag.Parse()

	options := cli.RunOptions{
		Strategy: bot.NewSimpleStrategy(),
	}
	if seed.set {
		options.Deal = domain.SeededDealOptions(seed.value)
	}
	if eventLogPath != "" {
		store, err := storage.NewJSONLEventStore(eventLogPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "durak: %v\n", err)
			os.Exit(1)
		}
		if matchID == "" {
			generatedID, err := newMatchID()
			if err != nil {
				fmt.Fprintf(os.Stderr, "durak: %v\n", err)
				os.Exit(1)
			}
			matchID = string(generatedID)
		}
		options.EventStore = store
		options.MatchID = app.MatchID(matchID)
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

func newMatchID() (app.MatchID, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate match id: %w", err)
	}
	return app.MatchID("cli-" + hex.EncodeToString(bytes[:])), nil
}
