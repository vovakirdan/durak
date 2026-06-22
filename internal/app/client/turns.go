package client

import (
	"fmt"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func (g *LocalGame) turnContext(seat domain.Seat) app.TurnContext {
	return app.TurnContext{
		SeriesID:        g.series.ID(),
		MatchID:         g.matchID,
		MatchNumber:     g.matchNumber,
		TurnNumber:      g.turnNumber,
		CanConcede:      true,
		DecisionContext: g.session.DecisionContext(seat),
	}
}

func (g *LocalGame) activeControllerSeat() domain.Seat {
	view := g.session.ViewForSeat(g.humanSeat)
	switch view.Phase {
	case domain.MatchPhaseAttack:
		return controllerSeat(view.Attacker, g.humanSeat)
	case domain.MatchPhaseDefense:
		return controllerSeat(view.Defender, g.humanSeat)
	case domain.MatchPhaseThrowIn, domain.MatchPhaseTaking:
		for _, seat := range throwInPollingOrder(view.Attacker, len(view.HandSizes)) {
			if seat == g.humanSeat || seat == view.Defender {
				continue
			}
			if len(g.session.DecisionContext(seat).LegalActions) > 0 {
				return seat
			}
		}
		return domain.NoSeat
	default:
		return domain.NoSeat
	}
}

func controllerSeat(seat, humanSeat domain.Seat) domain.Seat {
	if seat == humanSeat {
		return domain.NoSeat
	}
	return seat
}

func throwInPollingOrder(attacker domain.Seat, playerCount int) []domain.Seat {
	if playerCount <= 0 {
		return nil
	}
	order := make([]domain.Seat, 0, playerCount)
	start := ((int(attacker) % playerCount) + playerCount) % playerCount
	for offset := range playerCount {
		order = append(order, domain.Seat((start+offset)%playerCount))
	}
	return order
}

func localMatchID(base app.MatchID, matchNumber int) app.MatchID {
	if matchNumber == 1 {
		return base
	}
	return app.MatchID(fmt.Sprintf("%s-%d", base, matchNumber))
}
