package evaluation

import (
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// BattleResolution is the boundary-normalized result used for action scoring.
type BattleResolution struct {
	Context       app.DecisionContext
	Battle        BattleRisk
	FirstResponse BattleBranch
}

// ResolveBattleExpected applies an action and resolves the current battle to a
// common round boundary with the defender's lowest-risk branch.
func ResolveBattleExpected(
	decision *app.DecisionContext,
	hidden HiddenCards,
	action domain.Action,
) BattleResolution {
	if decision == nil {
		return BattleResolution{}
	}
	projected := projectAction(decision, action)
	hidden = ensureHiddenCards(&projected, hidden)
	resolution := BattleResolution{Context: projected}
	maxTransfers := max(1, len(projected.HandSizes)-1)
	for range maxTransfers + 1 {
		switch projected.Phase {
		case domain.MatchPhaseDefense:
			battle := EvaluateBattleRiskForSeat(&projected, hidden, projected.Defender)
			if resolution.FirstResponse == battleBranchNone {
				resolution.FirstResponse = battle.BestBranch
				resolution.Battle = battle
			}
			switch battle.BestBranch {
			case BattleBranchDefend:
				applyExpectedDefenseSuccess(&projected)
				resolution.Context = projected
				return resolution
			case BattleBranchTransfer:
				if battle.BestAction == (domain.Action{}) {
					projectFinishTake(&projected)
					resolution.Context = projected
					return resolution
				}
				projected = projectAction(&projected, battle.BestAction)
				hidden = BuildHiddenCards(&projected, nil)
			default:
				projectFinishTake(&projected)
				resolution.Context = projected
				return resolution
			}
		case domain.MatchPhaseTaking:
			projectFinishTake(&projected)
			if resolution.FirstResponse == battleBranchNone {
				resolution.FirstResponse = BattleBranchTake
			}
			resolution.Context = projected
			return resolution
		case domain.MatchPhaseThrowIn:
			projectFinishDefense(&projected)
			if resolution.FirstResponse == battleBranchNone {
				resolution.FirstResponse = BattleBranchDefend
			}
			resolution.Context = projected
			return resolution
		default:
			resolution.Context = projected
			return resolution
		}
	}
	projectFinishTake(&projected)
	resolution.Context = projected
	return resolution
}

func applyExpectedDefenseSuccess(projected *app.DecisionContext) {
	pending := pendingAttacks(projected.Table)
	for index := range projected.Table {
		if !projected.Table[index].Defended {
			projected.Table[index].Defended = true
		}
	}
	changeHandSize(projected, projected.Defender, -len(pending))
	projected.DiscardCount += len(pending)
	projected.Phase = domain.MatchPhaseThrowIn
	projectFinishDefense(projected)
}
