package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/adapters/cli"
	"github.com/vovakirdan/durak/internal/adapters/storage"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "durak: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	if len(args) > 0 && args[0] == "arena" {
		return runArena(ctx, args[1:], out, errOut)
	}
	return runPlay(ctx, args, in, out, errOut)
}

func runPlay(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	var seed seedFlag
	var eventLogPath string
	var matchID string
	flags := flag.NewFlagSet("durak", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.Var(&seed, "seed", "deterministic deal seed for replayable games")
	flags.StringVar(&eventLogPath, "event-log", "", "append public match events to a JSONL file")
	flags.StringVar(&matchID, "match-id", "", "base match id for event log; generated when omitted")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unknown argument %q", flags.Arg(0))
	}

	options := cli.RunOptions{
		Strategy: bot.NewSimpleStrategy(),
	}
	if seed.set {
		options.Deal = domain.SeededDealOptions(seed.value)
	}
	if eventLogPath != "" {
		store, err := storage.NewJSONLEventStore(eventLogPath)
		if err != nil {
			return err
		}
		if matchID == "" {
			generatedID, err := newMatchID()
			if err != nil {
				return err
			}
			matchID = string(generatedID)
		}
		options.EventStore = store
		options.MatchID = app.MatchID(matchID)
	}

	return cli.RunWithOptions(ctx, in, out, &options)
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
