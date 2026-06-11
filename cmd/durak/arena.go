package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sync"

	"github.com/vovakirdan/durak/internal/adapters/ai"
	"github.com/vovakirdan/durak/internal/adapters/storage"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

const (
	defaultArenaMatches    = 10
	defaultArenaSeed       = uint64(42)
	defaultArenaMaxActions = 500
)

type arenaOptions struct {
	matches      int
	seed         uint64
	maxActions   int
	eventLogPath string
	baseMatchID  string
	player0      string
	player1      string
	rules        string
}

type arenaSummary struct {
	matches       int
	turns         int
	wins          map[domain.Seat]int
	draws         int
	rawAIAttempts int
	rawAIInvalid  int
}

func runArena(ctx context.Context, args []string, out, errOut io.Writer) error {
	options, err := parseArenaOptions(args, errOut)
	if err != nil {
		return err
	}
	rawAIStats := &arenaRawAIStats{}
	result, err := runArenaMatches(ctx, &options, rawAIStats)
	if err != nil {
		return err
	}
	return writeArenaSummary(out, &options, summarizeArena(result, rawAIStats.summary()))
}

func parseArenaOptions(args []string, errOut io.Writer) (arenaOptions, error) {
	options := arenaOptions{
		matches:    defaultArenaMatches,
		seed:       defaultArenaSeed,
		maxActions: defaultArenaMaxActions,
		player0:    normalizePlayerControllerKind(""),
		player1:    normalizePlayerControllerKind(""),
		rules:      defaultRulesPreset,
	}
	flags := flag.NewFlagSet("durak arena", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.IntVar(&options.matches, "matches", options.matches, "number of headless matches to run")
	flags.Uint64Var(&options.seed, "seed", options.seed, "deterministic arena seed")
	flags.IntVar(&options.maxActions, "max-actions", options.maxActions, "maximum accepted actions per match")
	flags.StringVar(&options.eventLogPath, "event-log", "", "append public arena events to a JSONL file")
	flags.StringVar(&options.baseMatchID, "match-id", "", "base match id for event log")
	flags.StringVar(&options.player0, "p0", options.player0, "seat 0 controller: "+controllerNames())
	flags.StringVar(&options.player1, "p1", options.player1, "seat 1 controller: "+controllerNames())
	flags.StringVar(&options.rules, "rules", options.rules, "rule preset: default")
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
	if err := validatePlayerControllerKind(options.player0); err != nil {
		return arenaOptions{}, fmt.Errorf("p0: %w", err)
	}
	if err := validatePlayerControllerKind(options.player1); err != nil {
		return arenaOptions{}, fmt.Errorf("p1: %w", err)
	}
	if _, err := ruleProfile(options.rules); err != nil {
		return arenaOptions{}, err
	}
	return options, nil
}

func runArenaMatches(
	ctx context.Context,
	options *arenaOptions,
	rawAITraceSink ai.RawCommandTraceSink,
) (app.SeriesRunResult, error) {
	profile, err := ruleProfile(options.rules)
	if err != nil {
		return app.SeriesRunResult{}, err
	}
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "arena-series",
		Seats:    []domain.Seat{0, 1},
		Profile:  profile,
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

	player0, err := newPlayerController(playerControllerConfig{
		Kind:      options.player0,
		Seed:      options.seed,
		Seeded:    true,
		Seat:      domain.Seat(0),
		TraceSink: rawAITraceSink,
	})
	if err != nil {
		return app.SeriesRunResult{}, err
	}
	player1, err := newPlayerController(playerControllerConfig{
		Kind:      options.player1,
		Seed:      options.seed,
		Seeded:    true,
		Seat:      domain.Seat(1),
		TraceSink: rawAITraceSink,
	})
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

func summarizeArena(result app.SeriesRunResult, rawAI arenaRawAISummary) arenaSummary {
	summary := arenaSummary{
		matches:       len(result.Matches),
		turns:         len(result.Turns),
		wins:          make(map[domain.Seat]int),
		rawAIAttempts: rawAI.attempts,
		rawAIInvalid:  rawAI.invalid,
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
		"Arena: seat0=%s seat1=%s\nRules: %s\nMatches: %d\nSeed: %d\nMax actions/match: %d\nTurns: %d\nResults: seat0=%d seat1=%d draws=%d\n",
		options.player0,
		options.player1,
		options.rules,
		summary.matches,
		options.seed,
		options.maxActions,
		summary.turns,
		summary.wins[domain.Seat(0)],
		summary.wins[domain.Seat(1)],
		summary.draws,
	)
	if err != nil {
		return err
	}
	if summary.rawAIAttempts == 0 {
		return nil
	}
	_, err = fmt.Fprintf(
		out,
		"Raw AI: attempts=%d invalid=%d\n",
		summary.rawAIAttempts,
		summary.rawAIInvalid,
	)
	return err
}

type arenaRawAIStats struct {
	mu       sync.Mutex
	attempts int
	invalid  int
}

type arenaRawAISummary struct {
	attempts int
	invalid  int
}

func (s *arenaRawAIStats) RecordRawCommandTrace(trace *ai.RawCommandTrace) {
	if s == nil || trace == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts++
	if trace.Err != "" {
		s.invalid++
	}
}

func (s *arenaRawAIStats) summary() arenaRawAISummary {
	if s == nil {
		return arenaRawAISummary{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return arenaRawAISummary{
		attempts: s.attempts,
		invalid:  s.invalid,
	}
}
