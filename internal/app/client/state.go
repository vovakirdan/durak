package client

import (
	"fmt"
	"strconv"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// Card is a transport-friendly card DTO.
type Card struct {
	Code string
	Rank string
	Suit string
}

// TablePair is one attack card and its optional defense card.
type TablePair struct {
	Attack  Card
	Defense *Card
}

// LegalAction is a stable client command candidate for the current seat.
type LegalAction struct {
	ID          string
	Kind        string
	Label       string
	Card        *Card
	AttackIndex int
}

// State is the game state a CLI, TUI, or protocol client can render.
type State struct {
	MatchID            string
	Version            uint64
	Seat               int
	Phase              string
	Attacker           int
	Defender           int
	TrumpSuit          string
	TrumpIndicator     Card
	Table              []TablePair
	Hand               []Card
	HandSizes          []int
	StockCount         int
	DiscardCount       int
	SuccessfulDefenses int
	Winner             int
	Loser              int
	LegalActions       []LegalAction
	Result             string
}

// StateFromDecision projects an app decision context into client DTOs.
func StateFromDecision(matchID app.MatchID, version uint64, decision *app.DecisionContext) State {
	if decision == nil {
		return State{MatchID: string(matchID), Version: version}
	}
	return State{
		MatchID:            string(matchID),
		Version:            version,
		Seat:               int(decision.Seat),
		Phase:              phaseString(decision.Phase),
		Attacker:           int(decision.Attacker),
		Defender:           int(decision.Defender),
		TrumpSuit:          decision.TrumpSuit.String(),
		TrumpIndicator:     cardDTO(decision.TrumpIndicator),
		Table:              tableDTO(decision.Table),
		Hand:               cardsDTO(decision.Hand),
		HandSizes:          append([]int(nil), decision.HandSizes...),
		StockCount:         decision.StockCount,
		DiscardCount:       decision.DiscardCount,
		SuccessfulDefenses: decision.SuccessfulDefenses,
		Winner:             int(decision.Winner),
		Loser:              int(decision.Loser),
		LegalActions:       legalActionsDTO(decision.LegalActions),
		Result:             resultString(&decision.SeatView),
	}
}

func tableDTO(table []domain.TablePair) []TablePair {
	if len(table) == 0 {
		return nil
	}
	pairs := make([]TablePair, 0, len(table))
	for _, pair := range table {
		item := TablePair{Attack: cardDTO(pair.Attack)}
		if pair.Defended {
			defense := cardDTO(pair.Defense)
			item.Defense = &defense
		}
		pairs = append(pairs, item)
	}
	return pairs
}

func cardsDTO(cards []domain.Card) []Card {
	if len(cards) == 0 {
		return nil
	}
	out := make([]Card, 0, len(cards))
	for _, card := range cards {
		out = append(out, cardDTO(card))
	}
	return out
}

func cardDTO(card domain.Card) Card {
	return Card{
		Code: card.String(),
		Rank: card.Rank.String(),
		Suit: card.Suit.String(),
	}
}

func legalActionsDTO(actions []domain.Action) []LegalAction {
	if len(actions) == 0 {
		return nil
	}
	out := make([]LegalAction, 0, len(actions))
	for index, action := range actions {
		item := LegalAction{
			ID:          strconv.Itoa(index + 1),
			Kind:        actionKindString(action.Kind),
			Label:       actionLabel(action),
			AttackIndex: action.AttackIndex,
		}
		if action.Card != (domain.Card{}) {
			card := cardDTO(action.Card)
			item.Card = &card
		}
		out = append(out, item)
	}
	return out
}

func phaseString(phase domain.MatchPhase) string {
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

func actionKindString(kind domain.ActionKind) string {
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

func actionLabel(action domain.Action) string {
	switch action.Kind {
	case domain.ActionKindAttack:
		return fmt.Sprintf("attack %s", action.Card)
	case domain.ActionKindDefend:
		return fmt.Sprintf("defend %d %s", action.AttackIndex+1, action.Card)
	case domain.ActionKindThrowIn:
		return fmt.Sprintf("throw %s", action.Card)
	case domain.ActionKindPassThrowIn:
		return "pass"
	case domain.ActionKindTake:
		return "take"
	case domain.ActionKindFinishDefense:
		return "done"
	case domain.ActionKindFinishTake:
		return "done"
	case domain.ActionKindTransfer:
		return fmt.Sprintf("transfer %s", action.Card)
	default:
		return "unknown"
	}
}

func resultString(view *app.SeatView) string {
	if view == nil || view.Phase != domain.MatchPhaseComplete {
		return ""
	}
	if view.Winner == domain.NoSeat || view.Loser == domain.NoSeat {
		return "draw"
	}
	return fmt.Sprintf("winner:%d loser:%d", view.Winner, view.Loser)
}
