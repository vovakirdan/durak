package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

type arenaEvaluationController struct {
	seat       domain.Seat
	controller app.PlayerController
	stats      *arenaEvaluationStats
}

type arenaEvaluationStats struct {
	mu      sync.Mutex
	overall arenaEvaluationCounter
	seats   map[domain.Seat]arenaEvaluationCounter
}

type arenaEvaluationSummary struct {
	overall arenaEvaluationCounter
	seats   map[domain.Seat]arenaEvaluationCounter
}

type arenaEvaluationCounter struct {
	turns      int
	lossTotal  int
	best       int
	good       int
	inaccuracy int
	mistake    int
	blunder    int
	illegal    int
}

func newArenaEvaluationStats(enabled bool) *arenaEvaluationStats {
	if !enabled {
		return nil
	}
	return &arenaEvaluationStats{seats: make(map[domain.Seat]arenaEvaluationCounter)}
}

func wrapArenaEvaluationController(
	seat domain.Seat,
	controller app.PlayerController,
	stats *arenaEvaluationStats,
) app.PlayerController {
	if stats == nil || controller == nil {
		return controller
	}
	return arenaEvaluationController{
		seat:       seat,
		controller: controller,
		stats:      stats,
	}
}

func (c arenaEvaluationController) Decide(ctx context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
	var observed app.TurnContext
	if turn != nil {
		observed = turn.Clone()
	}
	decision, err := c.controller.Decide(ctx, turn)
	if err != nil {
		return app.PlayerDecision{}, err
	}
	c.stats.record(c.seat, &observed, decision)
	return decision, nil
}

func (s *arenaEvaluationStats) record(
	seat domain.Seat,
	turn *app.TurnContext,
	decision app.PlayerDecision,
) {
	if s == nil || turn == nil || decision.Kind != app.PlayerDecisionAction {
		return
	}
	hidden := evaluation.BuildHiddenCards(&turn.DecisionContext, nil)
	actions := evaluation.RankActions(&turn.DecisionContext, hidden)
	result := findArenaActionEvaluation(actions, decision.Action)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.overall.record(result)
	seatCounter := s.seats[seat]
	seatCounter.record(result)
	s.seats[seat] = seatCounter
}

func (s *arenaEvaluationStats) summary() arenaEvaluationSummary {
	if s == nil {
		return arenaEvaluationSummary{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	seats := make(map[domain.Seat]arenaEvaluationCounter, len(s.seats))
	for seat, counter := range s.seats {
		seats[seat] = counter
	}
	return arenaEvaluationSummary{
		overall: s.overall,
		seats:   seats,
	}
}

func (c *arenaEvaluationCounter) record(result *evaluation.ActionEvaluation) {
	c.turns++
	if result == nil {
		c.illegal++
		return
	}
	c.lossTotal += int(result.Loss)
	switch result.Quality {
	case evaluation.MoveQualityBest:
		c.best++
	case evaluation.MoveQualityGood:
		c.good++
	case evaluation.MoveQualityInaccuracy:
		c.inaccuracy++
	case evaluation.MoveQualityMistake:
		c.mistake++
	case evaluation.MoveQualityBlunder:
		c.blunder++
	}
}

func writeArenaEvaluationSummary(
	out io.Writer,
	options *arenaOptions,
	summary *arenaEvaluationSummary,
) error {
	if summary == nil || summary.overall.turns == 0 {
		return nil
	}
	_, err := fmt.Fprintf(
		out,
		"Evaluation: turns=%d avg_loss=%d best=%d good=%d inaccuracy=%d mistake=%d blunder=%d illegal=%d seats=%s\n",
		summary.overall.turns,
		summary.overall.averageLoss(),
		summary.overall.best,
		summary.overall.good,
		summary.overall.inaccuracy,
		summary.overall.mistake,
		summary.overall.blunder,
		summary.overall.illegal,
		formatArenaEvaluationSeats(options, summary),
	)
	return err
}

func formatArenaEvaluationSeats(options *arenaOptions, summary *arenaEvaluationSummary) string {
	if options == nil || summary == nil {
		return "[]"
	}
	parts := make([]string, 0, options.seats)
	for seat := range options.seats {
		counter := summary.seats[domain.Seat(seat)]
		parts = append(parts, fmt.Sprintf(
			"%d:turns=%d avg_loss=%d blunder=%d",
			seat,
			counter.turns,
			counter.averageLoss(),
			counter.blunder,
		))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (c arenaEvaluationCounter) averageLoss() int {
	if c.turns == 0 {
		return 0
	}
	return c.lossTotal / c.turns
}

func findArenaActionEvaluation(
	actions []evaluation.ActionEvaluation,
	action domain.Action,
) *evaluation.ActionEvaluation {
	for index := range actions {
		if actions[index].Action == action {
			return &actions[index]
		}
	}
	return nil
}
