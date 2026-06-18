package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
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
	log     io.Writer
	logOn   bool
	logErr  error
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

func newArenaEvaluationStats(enabled bool, log io.Writer) *arenaEvaluationStats {
	if !enabled {
		return nil
	}
	return &arenaEvaluationStats{
		seats: make(map[domain.Seat]arenaEvaluationCounter),
		log:   log,
		logOn: log != nil,
	}
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
	if s.logOn {
		s.writeLog(turn, decision.Action, actions)
	}
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

func (s *arenaEvaluationStats) err() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logErr
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

type arenaEvaluationLogRow struct {
	MatchID           string `json:"match_id"`
	Turn              int    `json:"turn"`
	Seat              int    `json:"seat"`
	Phase             string `json:"phase"`
	RiskScore         int    `json:"risk_score"`
	BattleTake        string `json:"battle_take,omitempty"`
	BattleDefend      string `json:"battle_defend,omitempty"`
	BattleTransfer    string `json:"battle_transfer,omitempty"`
	CoverProbability  string `json:"cover_probability,omitempty"`
	LegalHorizonClass string `json:"legal_horizon_class"`
	LegalHorizonSet   string `json:"legal_horizon_set"`
	ChosenAction      string `json:"chosen_action"`
	ChosenKind        string `json:"chosen_kind"`
	TopAction         string `json:"top_action"`
	TopKind           string `json:"top_kind"`
	RankChosen        int    `json:"rank_chosen"`
	LossChosen        int    `json:"loss_chosen"`
}

func (s *arenaEvaluationStats) writeLog(
	turn *app.TurnContext,
	chosen domain.Action,
	actions []evaluation.ActionEvaluation,
) {
	if s.log == nil || s.logErr != nil || turn == nil || len(actions) == 0 {
		return
	}
	rankChosen, lossChosen := rankAndLoss(actions, chosen)
	position := evaluation.Evaluate(&turn.DecisionContext, evaluation.BuildHiddenCards(&turn.DecisionContext, nil))
	battle := evaluation.EvaluateBattleRisk(&turn.DecisionContext, evaluation.BuildHiddenCards(&turn.DecisionContext, nil))
	row := arenaEvaluationLogRow{
		MatchID:           string(turn.MatchID),
		Turn:              turn.TurnNumber,
		Seat:              int(turn.Seat),
		Phase:             phaseName(turn.Phase),
		RiskScore:         int(position.Score),
		BattleTake:        formatOptionalFloat(battle.TakeNow),
		BattleDefend:      formatOptionalFloat(battle.ContinueDefense),
		BattleTransfer:    formatOptionalFloat(battle.Transfer),
		CoverProbability:  formatOptionalFloat(battle.CoverProbability),
		LegalHorizonClass: legalHorizonClass(turn.LegalActions),
		LegalHorizonSet:   legalHorizonSet(turn.LegalActions),
		ChosenAction:      formatAnalyzeAction(chosen),
		ChosenKind:        actionKindName(chosen.Kind),
		TopAction:         formatAnalyzeAction(actions[0].Action),
		TopKind:           actionKindName(actions[0].Action.Kind),
		RankChosen:        rankChosen,
		LossChosen:        int(lossChosen),
	}
	data, err := json.Marshal(row)
	if err == nil {
		_, err = fmt.Fprintln(s.log, string(data))
	}
	if err != nil {
		s.logErr = fmt.Errorf("write eval log log_on=%t log_nil=%t: %w", s.logOn, s.log == nil, err)
	}
}

func formatOptionalFloat(value float64) string {
	if value == 0 || value > 1e100 {
		return ""
	}
	return fmt.Sprintf("%.3f", value)
}

func rankAndLoss(actions []evaluation.ActionEvaluation, action domain.Action) (int, evaluation.Score) {
	for index := range actions {
		if actions[index].Action == action {
			return index + 1, actions[index].Loss
		}
	}
	return 0, 0
}

func legalHorizonClass(actions []domain.Action) string {
	if len(actions) == 0 {
		return "empty"
	}
	first := actionHorizon(actions[0].Kind)
	for _, action := range actions[1:] {
		if actionHorizon(action.Kind) != first {
			return "heterogeneous"
		}
	}
	return "homogeneous"
}

func legalHorizonSet(actions []domain.Action) string {
	var classes []string
	for _, action := range actions {
		class := actionHorizon(action.Kind)
		if !slices.Contains(classes, class) {
			classes = append(classes, class)
		}
	}
	slices.Sort(classes)
	return strings.Join(classes, ",")
}

func actionHorizon(kind domain.ActionKind) string {
	switch kind {
	case domain.ActionKindAttack, domain.ActionKindThrowIn:
		return "mixed"
	case domain.ActionKindTake, domain.ActionKindFinishDefense, domain.ActionKindFinishTake:
		return "boundary"
	case domain.ActionKindDefend, domain.ActionKindTransfer, domain.ActionKindPassThrowIn:
		return "mid"
	default:
		return "unknown"
	}
}

func phaseName(phase domain.MatchPhase) string {
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

func actionKindName(kind domain.ActionKind) string {
	switch kind {
	case domain.ActionKindAttack:
		return "attack"
	case domain.ActionKindDefend:
		return "defend"
	case domain.ActionKindThrowIn:
		return "throw_in"
	case domain.ActionKindPassThrowIn:
		return "pass_throw_in"
	case domain.ActionKindTake:
		return "take"
	case domain.ActionKindFinishDefense:
		return "finish_defense"
	case domain.ActionKindFinishTake:
		return "finish_take"
	case domain.ActionKindTransfer:
		return "transfer"
	default:
		return "unknown"
	}
}
