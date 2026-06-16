package evaluation

import (
	"context"
	"slices"
	"sort"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// MatchAnalysisReader reads durable data needed for heuristic move analysis.
type MatchAnalysisReader interface {
	InternalEventsForMatch(context.Context, app.MatchID) ([]app.InternalEvent, error)
	ConfigSnapshot(context.Context, string) (app.MatchConfigSnapshot, error)
}

// MatchAnalysis is a move-by-move heuristic report for one replayed match.
type MatchAnalysis struct {
	MatchID app.MatchID
	Moves   []MoveAnalysis
	Summary MoveSummary
}

// MoveAnalysis records the selected action's quality at one replayed turn.
type MoveAnalysis struct {
	Sequence     uint64
	TurnNumber   int
	Seat         domain.Seat
	Action       domain.Action
	BestAction   domain.Action
	Rank         int
	LegalActions int
	Score        Score
	Delta        Score
	Loss         Score
	Quality      MoveQuality
	Confidence   int
}

// MoveSummary aggregates move qualities for a match.
type MoveSummary struct {
	Moves        int
	Concessions  int
	Best         int
	Good         int
	Inaccuracies int
	Mistakes     int
	Blunders     int
	lossTotal    int
}

// AnalyzeMatchFromHistory reads and analyzes one stored match.
func AnalyzeMatchFromHistory(
	ctx context.Context,
	reader MatchAnalysisReader,
	matchID app.MatchID,
) (MatchAnalysis, error) {
	if reader == nil {
		return MatchAnalysis{}, app.ErrNilHistoryStore
	}
	events, err := reader.InternalEventsForMatch(ctx, matchID)
	if err != nil {
		return MatchAnalysis{}, err
	}
	profile, err := analysisRuleProfile(ctx, reader, events)
	if err != nil {
		return MatchAnalysis{}, err
	}
	return AnalyzeInternalEvents(events, profile)
}

// AverageLoss returns integer average loss over analyzed moves.
func (s MoveSummary) AverageLoss() int {
	if s.Moves == 0 {
		return 0
	}
	return s.lossTotal / s.Moves
}

// WorstMoves returns the highest-loss moves first.
func (a *MatchAnalysis) WorstMoves(limit int) []MoveAnalysis {
	if a == nil {
		return nil
	}
	moves := slices.Clone(a.Moves)
	sort.SliceStable(moves, func(i, j int) bool {
		if moves[i].Loss != moves[j].Loss {
			return moves[i].Loss > moves[j].Loss
		}
		return moves[i].Sequence < moves[j].Sequence
	})
	if limit >= 0 && limit < len(moves) {
		return moves[:limit]
	}
	return moves
}

func (s *MoveSummary) record(move *MoveAnalysis) {
	s.Moves++
	s.lossTotal += int(move.Loss)
	switch move.Quality {
	case MoveQualityBest:
		s.Best++
	case MoveQualityGood:
		s.Good++
	case MoveQualityInaccuracy:
		s.Inaccuracies++
	case MoveQualityMistake:
		s.Mistakes++
	case MoveQualityBlunder:
		s.Blunders++
	}
}
