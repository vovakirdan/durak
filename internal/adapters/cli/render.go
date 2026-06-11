package cli

import (
	"fmt"
	"strings"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

type renderer struct {
	humanSeat domain.Seat
	botSeat   domain.Seat
}

func newRenderer(humanSeat, botSeat domain.Seat) renderer {
	return renderer{
		humanSeat: humanSeat,
		botSeat:   botSeat,
	}
}

func (renderer) writeHelp(out *output) {
	out.println("Commands: number | a <card> | d [attack#] <card> | throw <card> | take | done | help | quit")
	out.println("Cards can be hand indexes or codes like 6C, 10D, AH.")
}

func (r renderer) writeState(out *output, decision *app.DecisionContext) {
	if decision == nil {
		out.println("State: unavailable")
		return
	}
	out.println()
	out.printf("Phase: %s | You: %s | Attacker: %s | Defender: %s\n",
		formatPhase(decision.Phase), r.formatSeat(decision.Seat), r.formatSeat(decision.Attacker), r.formatSeat(decision.Defender))
	out.printf("Trump: %s (%s) | Stock: %d | Discard: %d | Hands: %s\n",
		decision.TrumpSuit, decision.TrumpIndicator, decision.StockCount, decision.DiscardCount, r.formatHandSizes(decision.HandSizes))
	r.writeTable(out, decision.Table)
	r.writeHand(out, decision.Hand)
	r.writeActions(out, decision.LegalActions)
	r.writeResult(out, &decision.SeatView)
}

func (renderer) writeTable(out *output, table []domain.TablePair) {
	if len(table) == 0 {
		out.println("Table: empty")
		return
	}

	out.println("Table:")
	for i, pair := range table {
		if pair.Defended {
			out.printf("  %d. %s / %s\n", i+1, pair.Attack, pair.Defense)
			continue
		}
		out.printf("  %d. %s / --\n", i+1, pair.Attack)
	}
}

func (renderer) writeHand(out *output, hand []domain.Card) {
	if len(hand) == 0 {
		out.println("Hand: empty")
		return
	}

	out.print("Hand:")
	for i, card := range hand {
		out.printf(" %d:%s", i+1, card)
	}
	out.println()
}

func (renderer) writeActions(out *output, actions []domain.Action) {
	if len(actions) == 0 {
		out.println("Actions: none")
		return
	}

	out.println("Actions:")
	for i, action := range actions {
		out.printf("  %d. %s\n", i+1, formatAction(action))
	}
}

func (renderer) writeResult(out *output, view *app.SeatView) {
	if view == nil {
		return
	}
	if view.Phase != domain.MatchPhaseComplete {
		return
	}
	switch view.Winner {
	case domain.NoSeat:
		out.println("Result: draw")
	case view.Seat:
		out.println("Result: you won")
	default:
		out.println("Result: you lost")
	}
}

func (r renderer) formatSeat(seat domain.Seat) string {
	switch seat {
	case r.humanSeat:
		return fmt.Sprintf("you(%d)", seat)
	case r.botSeat:
		return fmt.Sprintf("bot(%d)", seat)
	default:
		return fmt.Sprintf("seat(%d)", seat)
	}
}

func (r renderer) formatHandSizes(sizes []int) string {
	parts := make([]string, len(sizes))
	for seat, size := range sizes {
		parts[seat] = fmt.Sprintf("%s:%d", r.formatSeat(domain.Seat(seat)), size)
	}
	return strings.Join(parts, " ")
}

func formatPhase(phase domain.MatchPhase) string {
	switch phase {
	case domain.MatchPhaseAttack:
		return "attack"
	case domain.MatchPhaseDefense:
		return "defense"
	case domain.MatchPhaseThrowIn:
		return "throw-in"
	case domain.MatchPhaseTaking:
		return "taking"
	case domain.MatchPhaseComplete:
		return "complete"
	default:
		return "unknown"
	}
}

func formatAction(action domain.Action) string {
	switch action.Kind {
	case domain.ActionKindAttack:
		return fmt.Sprintf("attack %s", action.Card)
	case domain.ActionKindDefend:
		return fmt.Sprintf("defend %d with %s", action.AttackIndex+1, action.Card)
	case domain.ActionKindThrowIn:
		return fmt.Sprintf("throw %s", action.Card)
	case domain.ActionKindTake:
		return "take"
	case domain.ActionKindFinishDefense:
		return "done"
	case domain.ActionKindFinishTake:
		return "pick up"
	default:
		return "unknown"
	}
}
