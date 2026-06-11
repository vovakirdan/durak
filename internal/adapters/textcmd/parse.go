package textcmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// Kind identifies a parsed terminal-style command.
type Kind uint8

const (
	// KindAction applies a legal domain action.
	KindAction Kind = iota
	// KindConcede gives up the current match.
	KindConcede
	// KindHelp requests command help.
	KindHelp
	// KindQuit exits the current local loop.
	KindQuit
)

// Command is the intent parsed from one text command.
type Command struct {
	Kind   Kind
	Action domain.Action
}

// FormatActionCommand returns the canonical text command for an action.
func FormatActionCommand(action domain.Action) string {
	switch action.Kind {
	case domain.ActionKindAttack:
		return "attack " + action.Card.String()
	case domain.ActionKindDefend:
		return fmt.Sprintf("defend %d %s", action.AttackIndex+1, action.Card)
	case domain.ActionKindThrowIn:
		return "throw " + action.Card.String()
	case domain.ActionKindTake:
		return "take"
	case domain.ActionKindFinishDefense, domain.ActionKindFinishTake:
		return "done"
	case domain.ActionKindTransfer:
		return "transfer " + action.Card.String()
	default:
		return ""
	}
}

// Parse converts a terminal-style command into a command intent.
func Parse(input string, decision *app.DecisionContext) (Command, error) {
	if decision == nil {
		return Command{}, commandError("missing decision context")
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return Command{}, commandError("empty command")
	}
	if IsQuit(input) {
		return Command{Kind: KindQuit}, nil
	}
	if IsHelp(input) {
		return Command{Kind: KindHelp}, nil
	}
	if IsConcede(input) {
		return Command{Kind: KindConcede}, nil
	}
	if action, ok := parseActionNumber(input, decision.LegalActions); ok {
		return Command{Kind: KindAction, Action: action}, nil
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
		return Command{}, commandError("unknown command")
	}
}

// IsQuit reports whether input is a loop-exit command.
func IsQuit(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "q", "quit", "exit":
		return true
	default:
		return false
	}
}

// IsHelp reports whether input requests command help.
func IsHelp(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "h", "help", "?":
		return true
	default:
		return false
	}
}

// IsConcede reports whether input gives up the current match.
func IsConcede(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "concede", "surrender", "ff":
		return true
	default:
		return false
	}
}

func parseActionNumber(input string, actions []domain.Action) (domain.Action, bool) {
	index, err := strconv.Atoi(input)
	if err != nil || index < 1 || index > len(actions) {
		return domain.Action{}, false
	}
	return actions[index-1], true
}

func parseCardAction(kind domain.ActionKind, args []string, decision *app.DecisionContext) (Command, error) {
	if len(args) != 1 {
		return Command{}, commandError("card command expects one card")
	}
	card, err := parseCardSelector(args[0], decision.Hand)
	if err != nil {
		return Command{}, err
	}
	return findLegalAction(decision.LegalActions, func(action domain.Action) bool {
		return action.Kind == kind && action.Card == card
	})
}

func parseDefendAction(args []string, decision *app.DecisionContext) (Command, error) {
	if len(args) < 1 || len(args) > 2 {
		return Command{}, commandError("defend expects card or attack number and card")
	}

	attackIndex := -1
	cardToken := args[0]
	if len(args) == 2 {
		index, err := strconv.Atoi(args[0])
		if err != nil || index < 1 {
			return Command{}, commandError("invalid attack number")
		}
		attackIndex = index - 1
		cardToken = args[1]
	}

	card, err := parseCardSelector(cardToken, decision.Hand)
	if err != nil {
		return Command{}, err
	}
	return findLegalAction(decision.LegalActions, func(action domain.Action) bool {
		if action.Kind != domain.ActionKindDefend || action.Card != card {
			return false
		}
		return attackIndex == -1 || action.AttackIndex == attackIndex
	})
}

func parseKindAction(kind domain.ActionKind, decision *app.DecisionContext) (Command, error) {
	return findLegalAction(decision.LegalActions, func(action domain.Action) bool {
		return action.Kind == kind
	})
}

func parseFinishAction(decision *app.DecisionContext) (Command, error) {
	if command, err := parseKindAction(domain.ActionKindFinishDefense, decision); err == nil {
		return command, nil
	}
	return parseKindAction(domain.ActionKindFinishTake, decision)
}

func findLegalAction(actions []domain.Action, match func(domain.Action) bool) (Command, error) {
	for _, action := range actions {
		if match(action) {
			return Command{Kind: KindAction, Action: action}, nil
		}
	}
	return Command{}, commandError("action is not legal now")
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

func commandError(message string) error {
	return errors.New(message)
}
