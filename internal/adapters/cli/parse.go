package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

type commandKind uint8

const (
	commandAction commandKind = iota
	commandConcede
	commandHelp
	commandQuit
)

type parsedCommand struct {
	kind   commandKind
	action domain.Action
}

func parseCommand(input string, decision *app.DecisionContext) (parsedCommand, error) {
	if decision == nil {
		return parsedCommand{}, commandError("missing decision context")
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return parsedCommand{}, commandError("empty command")
	}
	if isQuit(input) {
		return parsedCommand{kind: commandQuit}, nil
	}
	if isHelp(input) {
		return parsedCommand{kind: commandHelp}, nil
	}
	if isConcede(input) {
		return parsedCommand{kind: commandConcede}, nil
	}
	if action, ok := parseActionNumber(input, decision.LegalActions); ok {
		return parsedCommand{kind: commandAction, action: action}, nil
	}

	fields := strings.Fields(input)
	switch strings.ToLower(fields[0]) {
	case "a", "attack":
		return parseCardAction(domain.ActionKindAttack, fields[1:], decision)
	case "throw", "th", "add":
		return parseCardAction(domain.ActionKindThrowIn, fields[1:], decision)
	case "tr", "transfer":
		return parseCardAction(domain.ActionKindTransfer, fields[1:], decision)
	case "d", "defend":
		return parseDefendAction(fields[1:], decision)
	case "t", "take":
		return parseKindAction(domain.ActionKindTake, decision)
	case "f", "finish", "done", "pass":
		return parseFinishAction(decision)
	default:
		return parsedCommand{}, commandError("unknown command")
	}
}

func parseActionNumber(input string, actions []domain.Action) (domain.Action, bool) {
	index, err := strconv.Atoi(input)
	if err != nil || index < 1 || index > len(actions) {
		return domain.Action{}, false
	}
	return actions[index-1], true
}

func parseCardAction(kind domain.ActionKind, args []string, decision *app.DecisionContext) (parsedCommand, error) {
	if len(args) != 1 {
		return parsedCommand{}, commandError("card command expects one card")
	}
	card, err := parseCardSelector(args[0], decision.Hand)
	if err != nil {
		return parsedCommand{}, err
	}
	return findLegalAction(decision.LegalActions, func(action domain.Action) bool {
		return action.Kind == kind && action.Card == card
	})
}

func parseDefendAction(args []string, decision *app.DecisionContext) (parsedCommand, error) {
	if len(args) < 1 || len(args) > 2 {
		return parsedCommand{}, commandError("defend expects card or attack number and card")
	}

	attackIndex := -1
	cardToken := args[0]
	if len(args) == 2 {
		index, err := strconv.Atoi(args[0])
		if err != nil || index < 1 {
			return parsedCommand{}, commandError("invalid attack number")
		}
		attackIndex = index - 1
		cardToken = args[1]
	}

	card, err := parseCardSelector(cardToken, decision.Hand)
	if err != nil {
		return parsedCommand{}, err
	}
	return findLegalAction(decision.LegalActions, func(action domain.Action) bool {
		if action.Kind != domain.ActionKindDefend || action.Card != card {
			return false
		}
		return attackIndex == -1 || action.AttackIndex == attackIndex
	})
}

func parseKindAction(kind domain.ActionKind, decision *app.DecisionContext) (parsedCommand, error) {
	return findLegalAction(decision.LegalActions, func(action domain.Action) bool {
		return action.Kind == kind
	})
}

func parseFinishAction(decision *app.DecisionContext) (parsedCommand, error) {
	if command, err := parseKindAction(domain.ActionKindFinishDefense, decision); err == nil {
		return command, nil
	}
	return parseKindAction(domain.ActionKindFinishTake, decision)
}

func findLegalAction(actions []domain.Action, match func(domain.Action) bool) (parsedCommand, error) {
	for _, action := range actions {
		if match(action) {
			return parsedCommand{kind: commandAction, action: action}, nil
		}
	}
	return parsedCommand{}, commandError("action is not legal now")
}

func parseCardSelector(input string, hand []domain.Card) (domain.Card, error) {
	if index, err := strconv.Atoi(input); err == nil {
		if index < 1 || index > len(hand) {
			return domain.Card{}, commandError("card index is outside hand")
		}
		return hand[index-1], nil
	}

	normalized := strings.ToUpper(input)
	for _, card := range hand {
		if strings.EqualFold(card.String(), normalized) {
			return card, nil
		}
	}
	return domain.Card{}, fmt.Errorf("card %q is not in hand", input)
}
