package bot_test

import (
	"context"
	"testing"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestSimpleStrategyPlaysHeadlessSeries(t *testing.T) {
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "bot-smoke",
		Seats:    []domain.Seat{0, 1},
	})
	if err != nil {
		t.Fatalf("NewSeries returned error: %v", err)
	}
	controller := app.StrategyController{Strategy: bot.NewSimpleStrategy()}
	runner, err := app.NewSeriesRunner(&app.SeriesRunnerOptions{
		Series: series,
		Controllers: map[domain.Seat]app.PlayerController{
			domain.Seat(0): controller,
			domain.Seat(1): controller,
		},
		Deal:               domain.SeededDealOptions(42),
		BaseMatchID:        "bot-smoke-match",
		MaxActionsPerMatch: 300,
	})
	if err != nil {
		t.Fatalf("NewSeriesRunner returned error: %v", err)
	}

	result, err := runner.Run(context.Background(), 3)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.Matches) != 3 {
		t.Fatalf("matches = %+v, want three completed matches", result.Matches)
	}
	if len(result.Turns) == 0 {
		t.Fatal("turns are empty, want decision trace")
	}
}
