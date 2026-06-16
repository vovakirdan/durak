package evaluation

import (
	"context"
	"fmt"
	"reflect"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// AnalyzeInternalEvents reconstructs a match and scores each stored action.
func AnalyzeInternalEvents(events []app.InternalEvent, profile domain.RuleProfile) (MatchAnalysis, error) {
	if len(events) == 0 {
		return MatchAnalysis{}, fmt.Errorf("%w: event stream is empty", app.ErrInvalidReplay)
	}
	if err := validateAnalysisSequence(events); err != nil {
		return MatchAnalysis{}, err
	}

	analysis := MatchAnalysis{MatchID: events[0].MatchID}
	var match *domain.Match
	replayedEvents := make([]domain.Event, 0, len(events))
	for i := range events {
		event := events[i]
		nextMatch, replayed, err := analyzeInternalEvent(match, &event, &analysis, profile)
		if err != nil {
			return MatchAnalysis{}, err
		}
		match = nextMatch
		replayedEvents = append(replayedEvents, replayed...)
	}
	if match == nil {
		return MatchAnalysis{}, fmt.Errorf("%w: missing deal event", app.ErrInvalidReplay)
	}
	if !reflect.DeepEqual(replayedEvents, analysisSourceEvents(events)) {
		return MatchAnalysis{}, fmt.Errorf("%w: replayed events differ from source", app.ErrInvalidReplay)
	}
	return analysis, nil
}

func analyzeInternalEvent(
	match *domain.Match,
	event *app.InternalEvent,
	analysis *MatchAnalysis,
	profile domain.RuleProfile,
) (*domain.Match, []domain.Event, error) {
	switch event.Domain.Kind {
	case domain.EventKindMatchStarted:
		return match, nil, nil
	case domain.EventKindDeal:
		nextMatch, err := analysisDeal(match, event, profile)
		if err != nil {
			return nil, nil, err
		}
		replayed := nextMatch.DrainEvents()
		return nextMatch, replayed, nil
	case domain.EventKindAttack, domain.EventKindDefend, domain.EventKindThrowIn,
		domain.EventKindPassThrowIn, domain.EventKindTransfer, domain.EventKindTake,
		domain.EventKindFinishDefense, domain.EventKindFinishTake:
		replayed, err := analyzeAndApplyStoredAction(match, event, analysis)
		return match, replayed, err
	case domain.EventKindConcede:
		replayed, err := analyzeStoredConcede(match, event, analysis)
		return match, replayed, err
	case domain.EventKindRefill, domain.EventKindRoundEnded, domain.EventKindMatchEnded:
		return match, nil, nil
	default:
		return nil, nil, fmt.Errorf("%w: unknown event kind %d at sequence %d",
			app.ErrInvalidReplay, event.Domain.Kind, event.Sequence)
	}
}

func analysisDeal(
	current *domain.Match,
	event *app.InternalEvent,
	profile domain.RuleProfile,
) (*domain.Match, error) {
	if current != nil {
		return nil, fmt.Errorf("%w: duplicate deal event at sequence %d", app.ErrInvalidReplay, event.Sequence)
	}
	if event.Deal == nil {
		return nil, fmt.Errorf("%w: missing internal deal at sequence %d", app.ErrInvalidReplay, event.Sequence)
	}
	deal := event.Deal.InitialDeal()
	match, err := domain.NewMatch(&deal, profile)
	if err != nil {
		return nil, fmt.Errorf("%w: create match at sequence %d: %w", app.ErrInvalidReplay, event.Sequence, err)
	}
	return match, nil
}

func analyzeAndApplyStoredAction(
	match *domain.Match,
	event *app.InternalEvent,
	analysis *MatchAnalysis,
) ([]domain.Event, error) {
	move, err := analyzeStoredAction(match, event, len(analysis.Moves)+1)
	if err != nil {
		return nil, err
	}
	analysis.Moves = append(analysis.Moves, move)
	analysis.Summary.record(&move)
	if err := match.ApplyAction(event.Domain.Action.Action); err != nil {
		return nil, fmt.Errorf("%w: apply action at sequence %d: %w",
			app.ErrInvalidReplay, event.Sequence, err)
	}
	return match.DrainEvents(), nil
}

func analyzeStoredConcede(
	match *domain.Match,
	event *app.InternalEvent,
	analysis *MatchAnalysis,
) ([]domain.Event, error) {
	if match == nil {
		return nil, fmt.Errorf("%w: concede before deal at sequence %d",
			app.ErrInvalidReplay, event.Sequence)
	}
	if event.Domain.Concede == nil {
		return nil, fmt.Errorf("%w: missing concede payload at sequence %d",
			app.ErrInvalidReplay, event.Sequence)
	}
	analysis.Summary.Concessions++
	if err := match.Concede(event.Domain.Concede.Seat); err != nil {
		return nil, fmt.Errorf("%w: concede at sequence %d: %w",
			app.ErrInvalidReplay, event.Sequence, err)
	}
	return match.DrainEvents(), nil
}

func analyzeStoredAction(match *domain.Match, event *app.InternalEvent, turnNumber int) (MoveAnalysis, error) {
	if match == nil {
		return MoveAnalysis{}, fmt.Errorf("%w: action before deal at sequence %d",
			app.ErrInvalidReplay, event.Sequence)
	}
	if event.Domain.Action == nil {
		return MoveAnalysis{}, fmt.Errorf("%w: missing action payload at sequence %d",
			app.ErrInvalidReplay, event.Sequence)
	}
	action := event.Domain.Action.Action
	decision := decisionContextFromMatch(match, action.Seat)
	hidden := BuildHiddenCards(&decision, match.Discard())
	position := Evaluate(&decision, hidden)
	actions := RankActions(&decision, hidden)
	selected, rank := rankedAction(actions, action)
	if selected == nil {
		return MoveAnalysis{}, fmt.Errorf("%w: action at sequence %d is not legal",
			app.ErrInvalidReplay, event.Sequence)
	}
	return MoveAnalysis{
		Sequence:     event.Sequence,
		TurnNumber:   turnNumber,
		Seat:         action.Seat,
		Action:       action,
		BestAction:   actions[0].Action,
		Rank:         rank,
		LegalActions: len(actions),
		Score:        selected.Score,
		Delta:        selected.Delta,
		Loss:         selected.Loss,
		Quality:      selected.Quality,
		Confidence:   position.Confidence,
	}, nil
}

func rankedAction(actions []ActionEvaluation, action domain.Action) (selected *ActionEvaluation, rank int) {
	for index := range actions {
		if actions[index].Action == action {
			return &actions[index], index + 1
		}
	}
	return nil, 0
}

func decisionContextFromMatch(match *domain.Match, seat domain.Seat) app.DecisionContext {
	return app.DecisionContext{
		SeatView: app.SeatView{
			Seat:               seat,
			Phase:              match.Phase(),
			Attacker:           match.Attacker(),
			Defender:           match.Defender(),
			TrumpSuit:          match.TrumpSuit(),
			TrumpIndicator:     match.TrumpIndicator(),
			Table:              match.Table(),
			HandSizes:          analysisHandSizes(match),
			StockCount:         match.StockCount(),
			DiscardCount:       match.DiscardCount(),
			SuccessfulDefenses: match.SuccessfulDefenses(),
			Winner:             match.Winner(),
			Loser:              match.Loser(),
		},
		Hand:         match.Hand(seat),
		LegalActions: match.LegalActions(seat),
	}
}

func analysisHandSizes(match *domain.Match) []int {
	sizes := make([]int, match.PlayerCount())
	for seat := range sizes {
		sizes[seat] = match.HandSize(domain.Seat(seat))
	}
	return sizes
}

func analysisRuleProfile(
	ctx context.Context,
	reader MatchAnalysisReader,
	events []app.InternalEvent,
) (domain.RuleProfile, error) {
	if len(events) == 0 || events[0].ConfigIdentity.Hash == "" {
		return domain.DefaultRuleProfile(), nil
	}
	snapshot, err := reader.ConfigSnapshot(ctx, events[0].ConfigIdentity.Hash)
	if err != nil {
		return domain.RuleProfile{}, err
	}
	profile, err := snapshot.Config.RuleProfile()
	if err != nil {
		return domain.RuleProfile{}, fmt.Errorf("%w: analysis config profile: %w", app.ErrInvalidReplay, err)
	}
	return profile, nil
}

func validateAnalysisSequence(events []app.InternalEvent) error {
	matchID := events[0].MatchID
	if matchID == "" {
		return fmt.Errorf("%w: match id is empty", app.ErrInvalidReplay)
	}
	for i := range events {
		event := events[i]
		if event.MatchID != matchID {
			return fmt.Errorf("%w: event %d match id = %q, want %q",
				app.ErrInvalidReplay, i, event.MatchID, matchID)
		}
		sequence := uint64(i + 1)
		if event.Sequence != sequence {
			return fmt.Errorf("%w: event %d sequence = %d, want %d",
				app.ErrInvalidReplay, i, event.Sequence, sequence)
		}
	}
	return nil
}

func analysisSourceEvents(events []app.InternalEvent) []domain.Event {
	source := make([]domain.Event, len(events))
	for i := range events {
		source[i] = events[i].Domain.Clone()
	}
	return source
}
