package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/adapters/storage"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

const (
	defaultArenaMatches    = 10
	defaultArenaSeed       = uint64(42)
	defaultArenaMaxActions = 500

	arenaControllerSimple = "simple"
	arenaControllerRandom = "random"
)

type arenaOptions struct {
	matches      int
	seed         uint64
	maxActions   int
	eventLogPath string
	baseMatchID  string
	player0      string
	player1      string
}

type arenaSummary struct {
	matches int
	turns   int
	wins    map[domain.Seat]int
	draws   int
}

func runArena(ctx context.Context, args []string, out, errOut io.Writer) error {
	options, err := parseArenaOptions(args, errOut)
	if err != nil {
		return err
	}
	result, err := runArenaMatches(ctx, &options)
	if err != nil {
		return err
	}
	return writeArenaSummary(out, &options, summarizeArena(result))
}

func parseArenaOptions(args []string, errOut io.Writer) (arenaOptions, error) {
	options := arenaOptions{
		matches:    defaultArenaMatches,
		seed:       defaultArenaSeed,
		maxActions: defaultArenaMaxActions,
		player0:    arenaControllerSimple,
		player1:    arenaControllerSimple,
	}
	flags := flag.NewFlagSet("durak arena", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.IntVar(&options.matches, "matches", options.matches, "number of headless matches to run")
	flags.Uint64Var(&options.seed, "seed", options.seed, "deterministic arena seed")
	flags.IntVar(&options.maxActions, "max-actions", options.maxActions, "maximum accepted actions per match")
	flags.StringVar(&options.eventLogPath, "event-log", "", "append public arena events to a JSONL file")
	flags.StringVar(&options.baseMatchID, "match-id", "", "base match id for event log")
	flags.StringVar(&options.player0, "p0", options.player0, "seat 0 controller: simple or random")
	flags.StringVar(&options.player1, "p1", options.player1, "seat 1 controller: simple or random")
	if err := flags.Parse(args); err != nil {
		return arenaOptions{}, err
	}
	if flags.NArg() != 0 {
		return arenaOptions{}, fmt.Errorf("unknown arena argument %q", flags.Arg(0))
	}
	if options.matches <= 0 {
		return arenaOptions{}, fmt.Errorf("matches must be positive")
	}
	if options.maxActions <= 0 {
		return arenaOptions{}, fmt.Errorf("max-actions must be positive")
	}
	if err := validateArenaController(options.player0); err != nil {
		return arenaOptions{}, fmt.Errorf("p0: %w", err)
	}
	if err := validateArenaController(options.player1); err != nil {
		return arenaOptions{}, fmt.Errorf("p1: %w", err)
	}
	return options, nil
}

func runArenaMatches(ctx context.Context, options *arenaOptions) (app.SeriesRunResult, error) {
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "arena-series",
		Seats:    []domain.Seat{0, 1},
	})
	if err != nil {
		return app.SeriesRunResult{}, err
	}

	var eventStore app.EventStore
	if options.eventLogPath != "" {
		store, storeErr := storage.NewJSONLEventStore(options.eventLogPath)
		if storeErr != nil {
			return app.SeriesRunResult{}, storeErr
		}
		eventStore = store
	}

	player0, err := arenaController(options.player0, options.seed, domain.Seat(0))
	if err != nil {
		return app.SeriesRunResult{}, err
	}
	player1, err := arenaController(options.player1, options.seed, domain.Seat(1))
	if err != nil {
		return app.SeriesRunResult{}, err
	}

	runner, err := app.NewSeriesRunner(&app.SeriesRunnerOptions{
		Series: series,
		Controllers: map[domain.Seat]app.PlayerController{
			domain.Seat(0): player0,
			domain.Seat(1): player1,
		},
		Deal:               domain.SeededDealOptions(options.seed),
		EventStore:         eventStore,
		BaseMatchID:        app.MatchID(options.baseMatchID),
		MaxActionsPerMatch: options.maxActions,
	})
	if err != nil {
		return app.SeriesRunResult{}, err
	}
	return runner.Run(ctx, options.matches)
}

func validateArenaController(name string) error {
	switch name {
	case arenaControllerSimple, arenaControllerRandom:
		return nil
	default:
		return fmt.Errorf("unknown arena controller %q", name)
	}
}

func arenaController(name string, seed uint64, seat domain.Seat) (app.PlayerController, error) {
	switch name {
	case arenaControllerSimple:
		return app.StrategyController{Strategy: bot.NewSimpleStrategy()}, nil
	case arenaControllerRandom:
		salt, err := arenaSeatSeedSalt(seat)
		if err != nil {
			return nil, err
		}
		random := domain.NewSeededRandom(seed + salt)
		return bot.NewRandomLegalController(random.Intn), nil
	default:
		return nil, fmt.Errorf("unknown arena controller %q", name)
	}
}

func arenaSeatSeedSalt(seat domain.Seat) (uint64, error) {
	switch seat {
	case domain.Seat(0):
		return 1, nil
	case domain.Seat(1):
		return 2, nil
	default:
		return 0, fmt.Errorf("unsupported arena seat %d", seat)
	}
}

func summarizeArena(result app.SeriesRunResult) arenaSummary {
	summary := arenaSummary{
		matches: len(result.Matches),
		turns:   len(result.Turns),
		wins:    make(map[domain.Seat]int),
	}
	for _, match := range result.Matches {
		if match.Draw {
			summary.draws++
			continue
		}
		summary.wins[match.Winner]++
	}
	return summary
}

func writeArenaSummary(out io.Writer, options *arenaOptions, summary arenaSummary) error {
	_, err := fmt.Fprintf(
		out,
		"Arena: seat0=%s seat1=%s\nMatches: %d\nSeed: %d\nMax actions/match: %d\nTurns: %d\nResults: seat0=%d seat1=%d draws=%d\n",
		options.player0,
		options.player1,
		summary.matches,
		options.seed,
		options.maxActions,
		summary.turns,
		summary.wins[domain.Seat(0)],
		summary.wins[domain.Seat(1)],
		summary.draws,
	)
	return err
}
