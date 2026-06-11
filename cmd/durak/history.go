package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/vovakirdan/durak/internal/adapters/storage"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

type historyOptions struct {
	eventLogPath string
}

func runHistory(ctx context.Context, args []string, out, errOut io.Writer) error {
	options, err := parseHistoryOptions(args, errOut)
	if err != nil {
		return err
	}
	store, err := storage.NewJSONLEventStore(options.eventLogPath)
	if err != nil {
		return err
	}
	events, err := store.Events(ctx)
	if err != nil {
		return err
	}
	summaries, err := app.BuildMatchSummaries(events)
	if err != nil {
		return err
	}
	return writeHistory(out, summaries)
}

func parseHistoryOptions(args []string, errOut io.Writer) (historyOptions, error) {
	var options historyOptions
	flags := flag.NewFlagSet("durak history", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.StringVar(&options.eventLogPath, "event-log", "", "read public match events from a JSONL file")
	if err := flags.Parse(args); err != nil {
		return historyOptions{}, err
	}
	if flags.NArg() != 0 {
		return historyOptions{}, fmt.Errorf("unknown history argument %q", flags.Arg(0))
	}
	if options.eventLogPath == "" {
		return historyOptions{}, fmt.Errorf("event-log is required")
	}
	return options, nil
}

func writeHistory(out io.Writer, summaries []app.MatchSummary) error {
	if len(summaries) == 0 {
		_, err := fmt.Fprintln(out, "History: no matches")
		return err
	}
	if _, err := fmt.Fprintln(out, "History:"); err != nil {
		return err
	}
	for i := range summaries {
		if _, err := fmt.Fprintf(
			out,
			"- match=%s status=%s seats=%s rule=%s actions=%d result=%s\n",
			summaries[i].MatchID,
			historyStatus(&summaries[i]),
			formatHistorySeats(summaries[i].Seats),
			historyRuleProfile(&summaries[i]),
			summaries[i].ActionCount,
			historyResult(&summaries[i]),
		); err != nil {
			return err
		}
	}
	return nil
}

func historyStatus(summary *app.MatchSummary) string {
	if summary.Completed {
		return "complete"
	}
	return "in_progress"
}

func historyRuleProfile(summary *app.MatchSummary) string {
	if summary.RuleProfile == "" {
		return "unknown"
	}
	return summary.RuleProfile
}

func historyResult(summary *app.MatchSummary) string {
	if !summary.Completed {
		return "pending"
	}
	if summary.Draw {
		return "draw"
	}
	result := fmt.Sprintf("winner=%d loser=%d", summary.Winner, summary.Loser)
	if summary.ConcededBy != domain.NoSeat {
		return fmt.Sprintf("%s conceded_by=%d", result, summary.ConcededBy)
	}
	return result
}

func formatHistorySeats(seats []domain.Seat) string {
	var builder strings.Builder
	builder.WriteByte('[')
	for i, seat := range seats {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.Itoa(int(seat)))
	}
	builder.WriteByte(']')
	return builder.String()
}
