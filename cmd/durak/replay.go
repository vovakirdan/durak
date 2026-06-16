package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/vovakirdan/durak/internal/adapters/storage"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

type replayOptions struct {
	dbPath  string
	matchID string
}

func runReplay(ctx context.Context, args []string, out, errOut io.Writer) error {
	options, err := parseReplayOptions(args, errOut)
	if err != nil {
		return err
	}
	return withSQLiteStore(ctx, options.dbPath, func(store *storage.SQLiteStore) error {
		result, replayErr := app.ReplayMatchFromHistory(ctx, store, app.MatchID(options.matchID))
		if replayErr != nil {
			return replayErr
		}
		return writeReplay(out, options.matchID, &result)
	})
}

func parseReplayOptions(args []string, errOut io.Writer) (replayOptions, error) {
	var options replayOptions
	flags := flag.NewFlagSet("durak replay", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.StringVar(&options.dbPath, "db", "", "read durable match history from a SQLite database")
	flags.StringVar(&options.matchID, "match-id", "", "match id to replay")
	if err := flags.Parse(args); err != nil {
		return replayOptions{}, err
	}
	if flags.NArg() != 0 {
		return replayOptions{}, fmt.Errorf("unknown replay argument %q", flags.Arg(0))
	}
	if options.dbPath == "" {
		return replayOptions{}, fmt.Errorf("db is required")
	}
	if options.matchID == "" {
		return replayOptions{}, fmt.Errorf("match-id is required")
	}
	return options, nil
}

func writeReplay(out io.Writer, matchID string, result *app.ReplayResult) error {
	if result == nil || result.Match == nil {
		return fmt.Errorf("replay result is empty")
	}
	_, err := fmt.Fprintf(
		out,
		"Replay: match=%s events=%d phase=%s players=%d stock=%d discard=%d result=%s\n",
		matchID,
		len(result.Events),
		replayPhase(result.Match.Phase()),
		result.Match.PlayerCount(),
		result.Match.StockCount(),
		result.Match.DiscardCount(),
		replayResult(result.Match),
	)
	return err
}

func replayPhase(phase domain.MatchPhase) string {
	switch phase {
	case domain.MatchPhaseAttack:
		return "attack"
	case domain.MatchPhaseDefense:
		return "defense"
	case domain.MatchPhaseThrowIn:
		return "throw_in"
	case domain.MatchPhaseTaking:
		return "taking"
	case domain.MatchPhaseComplete:
		return "complete"
	default:
		return "unknown"
	}
}

func replayResult(match *domain.Match) string {
	if match.Phase() != domain.MatchPhaseComplete {
		return "pending"
	}
	if match.Winner() == domain.NoSeat && match.Loser() == domain.NoSeat {
		return "draw"
	}
	return fmt.Sprintf("winner=%d loser=%d", match.Winner(), match.Loser())
}
