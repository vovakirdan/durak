package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
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
	evaluation   bool
	eventLogPath string
	dbPath       string
	baseMatchID  string
	seats        int
	players      []string
	rules        string
	aiConfig     aiFlags
}

type arenaSummary struct {
	matches       int
	turns         int
	wins          map[domain.Seat]int
	draws         int
	rawAIAttempts int
	rawAIInvalid  int
	evaluation    arenaEvaluationSummary
}

func runArena(ctx context.Context, args []string, out, errOut io.Writer) error {
	options, err := parseArenaOptions(args, errOut)
	if err != nil {
		return err
	}
	rawAIStats := &arenaRawAIStats{}
	evaluationStats := newArenaEvaluationStats(options.evaluation)
	aiTraceSink, err := options.aiConfig.openTraceSink()
	if err != nil {
		return err
	}
	traceSink := ai.CombineTraceSinks(rawAIStats, aiTraceSink)
	result, err := runArenaMatches(ctx, &options, traceSink, evaluationStats)
	closeErr := closeAITraceSink(aiTraceSink, err)
	if closeErr != nil {
		return closeErr
	}
	summary := summarizeArena(result, rawAIStats.summary(), evaluationStats.summary())
	return writeArenaSummary(out, &options, &summary)
}

func parseArenaOptions(args []string, errOut io.Writer) (arenaOptions, error) {
	options := arenaOptions{
		matches:    defaultArenaMatches,
		seed:       defaultArenaSeed,
		maxActions: defaultArenaMaxActions,
		seats:      2,
		players:    defaultArenaPlayers(),
		rules:      defaultRulesPreset,
		aiConfig:   newAIFlags(),
	}
	flags := flag.NewFlagSet("durak arena", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.IntVar(&options.matches, "matches", options.matches, "number of headless matches to run")
	flags.Uint64Var(&options.seed, "seed", options.seed, "deterministic arena seed")
	flags.IntVar(&options.maxActions, "max-actions", options.maxActions, "maximum accepted actions per match")
	flags.BoolVar(&options.evaluation, "eval", options.evaluation, "print heuristic move-quality summary")
	flags.StringVar(&options.eventLogPath, "event-log", "", "append public arena events to a JSONL file")
	flags.StringVar(&options.dbPath, "db", "", "write durable match history to a SQLite database")
	flags.StringVar(&options.baseMatchID, "match-id", "", "base match id for stored match streams")
	flags.IntVar(&options.seats, "seats", options.seats, "number of active seats")
	for seat := range options.players {
		flags.StringVar(&options.players[seat], fmt.Sprintf("p%d", seat), options.players[seat],
			fmt.Sprintf("seat %d controller: %s", seat, controllerNames()))
	}
	flags.StringVar(&options.rules, "rules", options.rules, "rule preset: default")
	options.aiConfig.bind(flags)
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
	if _, err := matchConfig(options.rules, options.seats); err != nil {
		return arenaOptions{}, err
	}
	for seat := range options.seats {
		if err := validatePlayerControllerKind(options.players[seat]); err != nil {
			return arenaOptions{}, fmt.Errorf("p%d: %w", seat, err)
		}
	}
	return options, nil
}

func runArenaMatches(
	ctx context.Context,
	options *arenaOptions,
	rawAITraceSink ai.RawCommandTraceSink,
	evaluationStats *arenaEvaluationStats,
) (app.SeriesRunResult, error) {
	config, err := matchConfig(options.rules, options.seats)
	if err != nil {
		return app.SeriesRunResult{}, err
	}
	seats := arenaSeats(options.seats)
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "arena-series",
		Seats:    seats,
		Config:   config,
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
	var matchRecorder app.MatchRecorder
	var sqliteStore *storage.SQLiteStore
	if options.dbPath != "" {
		store, storeErr := storage.OpenSQLiteStore(ctx, options.dbPath)
		if storeErr != nil {
			return app.SeriesRunResult{}, storeErr
		}
		sqliteStore = store
		matchRecorder = store
	}

	controllers := make(map[domain.Seat]app.PlayerController, len(seats))
	for _, seat := range seats {
		kind := options.players[int(seat)]
		aiClient, clientErr := options.aiConfig.clientForKind(kind)
		if clientErr != nil {
			return app.SeriesRunResult{}, closeSQLiteStore(sqliteStore, clientErr)
		}
		controller, controllerErr := newPlayerController(&playerControllerConfig{
			Kind:      kind,
			Seed:      options.seed,
			Seeded:    true,
			Seat:      seat,
			TraceSink: rawAITraceSink,
			AI:        aiClient,
		})
		if controllerErr != nil {
			return app.SeriesRunResult{}, closeSQLiteStore(sqliteStore, controllerErr)
		}
		controller = wrapArenaEvaluationController(seat, controller, evaluationStats)
		controllers[seat] = controller
	}

	runner, err := app.NewSeriesRunner(&app.SeriesRunnerOptions{
		Series:             series,
		Controllers:        controllers,
		Deal:               domain.SeededDealOptions(options.seed),
		EventStore:         eventStore,
		MatchRecorder:      matchRecorder,
		BaseMatchID:        app.MatchID(options.baseMatchID),
		MaxActionsPerMatch: options.maxActions,
	})
	if err != nil {
		return app.SeriesRunResult{}, closeSQLiteStore(sqliteStore, err)
	}
	result, err := runner.Run(ctx, options.matches)
	return result, closeSQLiteStore(sqliteStore, err)
}

func summarizeArena(
	result app.SeriesRunResult,
	rawAI arenaRawAISummary,
	evaluation arenaEvaluationSummary,
) arenaSummary {
	summary := arenaSummary{
		matches:       len(result.Matches),
		turns:         len(result.Turns),
		wins:          make(map[domain.Seat]int),
		rawAIAttempts: rawAI.attempts,
		rawAIInvalid:  rawAI.invalid,
		evaluation:    evaluation,
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

func writeArenaSummary(out io.Writer, options *arenaOptions, summary *arenaSummary) error {
	_, err := fmt.Fprintf(
		out,
		"Arena: seat0=%s seat1=%s seats=%d players=%s\nRules: %s\nMatches: %d\nSeed: %d\nMax actions/match: %d\nTurns: %d\nResults: %s draws=%d\n",
		options.players[0],
		options.players[1],
		options.seats,
		formatArenaPlayers(options),
		options.rules,
		summary.matches,
		options.seed,
		options.maxActions,
		summary.turns,
		formatArenaResults(summary, options.seats),
		summary.draws,
	)
	if err != nil {
		return err
	}
	if summary.rawAIAttempts == 0 {
		return writeArenaEvaluationSummary(out, options, &summary.evaluation)
	}
	if _, err = fmt.Fprintf(
		out,
		"Raw AI: attempts=%d invalid=%d\n",
		summary.rawAIAttempts,
		summary.rawAIInvalid,
	); err != nil {
		return err
	}
	return writeArenaEvaluationSummary(out, options, &summary.evaluation)
}

func defaultArenaPlayers() []string {
	maxPlayers := domain.DefaultRuleProfile().MaxPlayers
	players := make([]string, maxPlayers)
	for seat := range players {
		players[seat] = normalizePlayerControllerKind("")
	}
	return players
}

func arenaSeats(count int) []domain.Seat {
	seats := make([]domain.Seat, count)
	for seat := range seats {
		seats[seat] = domain.Seat(seat)
	}
	return seats
}

func formatArenaPlayers(options *arenaOptions) string {
	parts := make([]string, 0, options.seats)
	for seat := range options.seats {
		parts = append(parts, fmt.Sprintf("%d:%s", seat, options.players[seat]))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func formatArenaResults(summary *arenaSummary, seats int) string {
	parts := make([]string, 0, seats)
	for seat := range seats {
		domainSeat := domain.Seat(seat)
		parts = append(parts, fmt.Sprintf("seat%d=%d", seat, summary.wins[domainSeat]))
	}
	return strings.Join(parts, " ")
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
