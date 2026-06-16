package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/vovakirdan/durak/internal/adapters/storage"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

const defaultAnalyzeLimit = 5

type analyzeOptions struct {
	dbPath  string
	matchID string
	limit   int
}

func runAnalyze(ctx context.Context, args []string, out, errOut io.Writer) error {
	options, err := parseAnalyzeOptions(args, errOut)
	if err != nil {
		return err
	}
	return withSQLiteStore(ctx, options.dbPath, func(store *storage.SQLiteStore) error {
		analysis, analyzeErr := evaluation.AnalyzeMatchFromHistory(ctx, store, app.MatchID(options.matchID))
		if analyzeErr != nil {
			return analyzeErr
		}
		return writeAnalyze(out, &options, &analysis)
	})
}

func parseAnalyzeOptions(args []string, errOut io.Writer) (analyzeOptions, error) {
	options := analyzeOptions{limit: defaultAnalyzeLimit}
	flags := flag.NewFlagSet("durak analyze", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.StringVar(&options.dbPath, "db", "", "read durable match history from a SQLite database")
	flags.StringVar(&options.matchID, "match-id", "", "match id to analyze")
	flags.IntVar(&options.limit, "limit", options.limit, "number of highest-loss moves to print")
	if err := flags.Parse(args); err != nil {
		return analyzeOptions{}, err
	}
	if flags.NArg() != 0 {
		return analyzeOptions{}, fmt.Errorf("unknown analyze argument %q", flags.Arg(0))
	}
	if options.dbPath == "" {
		return analyzeOptions{}, fmt.Errorf("db is required")
	}
	if options.matchID == "" {
		return analyzeOptions{}, fmt.Errorf("match-id is required")
	}
	if options.limit < 0 {
		return analyzeOptions{}, fmt.Errorf("limit must be non-negative")
	}
	return options, nil
}

func writeAnalyze(out io.Writer, options *analyzeOptions, analysis *evaluation.MatchAnalysis) error {
	if analysis == nil {
		return fmt.Errorf("analysis result is empty")
	}
	summary := analysis.Summary
	if _, err := fmt.Fprintf(
		out,
		"Analysis: match=%s moves=%d avg_loss=%d best=%d good=%d inaccuracy=%d mistake=%d blunder=%d concessions=%d\n",
		options.matchID,
		summary.Moves,
		summary.AverageLoss(),
		summary.Best,
		summary.Good,
		summary.Inaccuracies,
		summary.Mistakes,
		summary.Blunders,
		summary.Concessions,
	); err != nil {
		return err
	}
	return writeAnalyzeWorstMoves(out, analysis.WorstMoves(options.limit))
}

func writeAnalyzeWorstMoves(out io.Writer, moves []evaluation.MoveAnalysis) error {
	if len(moves) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(out, "Worst moves:"); err != nil {
		return err
	}
	for index := range moves {
		move := &moves[index]
		if _, err := fmt.Fprintf(
			out,
			"  turn=%d seq=%d seat=%d loss=%d quality=%s rank=%d/%d action=%s best=%s score=%d confidence=%d\n",
			move.TurnNumber,
			move.Sequence,
			move.Seat,
			move.Loss,
			move.Quality,
			move.Rank,
			move.LegalActions,
			formatAnalyzeAction(move.Action),
			formatAnalyzeAction(move.BestAction),
			move.Score,
			move.Confidence,
		); err != nil {
			return err
		}
	}
	return nil
}

func formatAnalyzeAction(action domain.Action) string {
	switch action.Kind {
	case domain.ActionKindAttack:
		return "attack " + action.Card.String()
	case domain.ActionKindDefend:
		return fmt.Sprintf("defend %d %s", action.AttackIndex+1, action.Card)
	case domain.ActionKindThrowIn:
		return "throw " + action.Card.String()
	case domain.ActionKindPassThrowIn:
		return "pass"
	case domain.ActionKindTake:
		return "take"
	case domain.ActionKindFinishDefense:
		return "done"
	case domain.ActionKindFinishTake:
		return "done"
	case domain.ActionKindTransfer:
		return "transfer " + action.Card.String()
	default:
		return "unknown"
	}
}
