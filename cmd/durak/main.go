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
	if len(args) > 0 && args[0] == "history" {
		return runHistory(ctx, args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "replay" {
		return runReplay(ctx, args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "analyze" {
		return runAnalyze(ctx, args[1:], out, errOut)
	}
	return runPlay(ctx, args, in, out, errOut)
}

func runPlay(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	var seed seedFlag
	var eventLogPath string
	var dbPath string
	var matchID string
	var seats int
	var humanSeat int
	aiConfig := newAIFlags()
	botName := normalizePlayerControllerKind("")
	players := defaultArenaPlayers()
	rulesName := defaultRulesPreset
	flags := flag.NewFlagSet("durak", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.Var(&seed, "seed", "deterministic deal seed for replayable games")
	flags.StringVar(&botName, "bot", botName, "bot controller: "+controllerNames())
	flags.IntVar(&seats, "seats", 2, "number of active seats")
	flags.IntVar(&humanSeat, "human-seat", 0, "local human seat index")
	for seat := range players {
		flags.StringVar(&players[seat], fmt.Sprintf("p%d", seat), "", fmt.Sprintf("seat %d controller override: %s", seat, controllerNames()))
	}
	flags.StringVar(&rulesName, "rules", rulesName, "rule preset: default")
	flags.StringVar(&eventLogPath, "event-log", "", "append public match events to a JSONL file")
	flags.StringVar(&dbPath, "db", "", "write durable match history to a SQLite database")
	flags.StringVar(&matchID, "match-id", "", "base match id for event log; generated when omitted")
	aiConfig.bind(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unknown argument %q", flags.Arg(0))
	}
	if err := validatePlayerControllerKind(botName); err != nil {
		return err
	}

	config, err := matchConfig(rulesName, seats)
	if err != nil {
		return err
	}
	if humanSeat < 0 || humanSeat >= seats {
		return fmt.Errorf("human-seat must be in range 0..%d", seats-1)
	}
	aiTraceSink, err := aiConfig.openTraceSink()
	if err != nil {
		return err
	}
	controllers, err := playControllers(&playControllerOptions{
		seats:     seats,
		humanSeat: domain.Seat(humanSeat),
		botName:   botName,
		players:   players,
		seed:      seed.value,
		seeded:    seed.set,
		aiConfig:  &aiConfig,
		traceSink: aiTraceSink,
	})
	if err != nil {
		return closeAITraceSink(aiTraceSink, err)
	}
	options := cli.RunOptions{
		PlayerCount: seats,
		HumanSeat:   domain.Seat(humanSeat),
		Config:      config,
		Controllers: controllers,
	}
	if seed.set {
		options.Deal = domain.SeededDealOptions(seed.value)
	}
	if eventLogPath != "" {
		store, err := storage.NewJSONLEventStore(eventLogPath)
		if err != nil {
			return closeAITraceSink(aiTraceSink, err)
		}
		if matchID == "" {
			generatedID, err := newMatchID()
			if err != nil {
				return closeAITraceSink(aiTraceSink, err)
			}
			matchID = string(generatedID)
		}
		options.EventStore = store
		options.MatchID = app.MatchID(matchID)
	}
	var sqliteStore *storage.SQLiteStore
	if dbPath != "" {
		store, err := storage.OpenSQLiteStore(ctx, dbPath)
		if err != nil {
			return closeAITraceSink(aiTraceSink, err)
		}
		sqliteStore = store
		if matchID == "" {
			generatedID, err := newMatchID()
			if err != nil {
				return closeAITraceSink(aiTraceSink, closeSQLiteStore(sqliteStore, err))
			}
			matchID = string(generatedID)
		}
		options.Recorder = sqliteStore
		options.MatchID = app.MatchID(matchID)
	}

	runErr := cli.RunWithOptions(ctx, in, out, &options)
	runErr = closeSQLiteStore(sqliteStore, runErr)
	return closeAITraceSink(aiTraceSink, runErr)
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
