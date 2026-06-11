package cli

import (
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestParseCommandSelectsNumberedLegalAction(t *testing.T) {
	second := domain.Action{
		Kind: domain.ActionKindAttack,
		Seat: defaultHumanSeat,
		Card: domain.Card{Rank: domain.Seven, Suit: domain.Clubs},
	}
	decision := app.DecisionContext{
		LegalActions: []domain.Action{
			{Kind: domain.ActionKindAttack, Seat: defaultHumanSeat, Card: domain.Card{Rank: domain.Six, Suit: domain.Clubs}},
			second,
		},
	}

	command, err := parseCommand("2", &decision)
	if err != nil {
		t.Fatalf("parseCommand returned error: %v", err)
	}
	if command.kind != commandAction || command.action != second {
		t.Fatalf("command = %+v, want second legal action", command)
	}
}

func TestParseCommandParsesAttackByHandIndex(t *testing.T) {
	card := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	decision := app.DecisionContext{
		Hand: []domain.Card{card},
		LegalActions: []domain.Action{
			{Kind: domain.ActionKindAttack, Seat: defaultHumanSeat, Card: card},
		},
	}

	command, err := parseCommand("a 1", &decision)
	if err != nil {
		t.Fatalf("parseCommand returned error: %v", err)
	}
	if command.action.Kind != domain.ActionKindAttack || command.action.Card != card {
		t.Fatalf("action = %+v, want attack with %v", command.action, card)
	}
}

func TestParseCommandParsesDefendByAttackNumberAndCardCode(t *testing.T) {
	card := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	decision := app.DecisionContext{
		Hand: []domain.Card{card},
		LegalActions: []domain.Action{
			{Kind: domain.ActionKindDefend, Seat: defaultHumanSeat, Card: card, AttackIndex: 1},
		},
	}

	command, err := parseCommand("defend 2 7c", &decision)
	if err != nil {
		t.Fatalf("parseCommand returned error: %v", err)
	}
	if command.action.Kind != domain.ActionKindDefend || command.action.AttackIndex != 1 || command.action.Card != card {
		t.Fatalf("action = %+v, want defend second attack", command.action)
	}
}

func TestParseCommandParsesTransferByCardCode(t *testing.T) {
	card := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	decision := app.DecisionContext{
		Hand: []domain.Card{card},
		LegalActions: []domain.Action{
			{Kind: domain.ActionKindTransfer, Seat: defaultHumanSeat, Card: card},
		},
	}

	command, err := parseCommand("tr 7c", &decision)
	if err != nil {
		t.Fatalf("parseCommand returned error: %v", err)
	}
	if command.action.Kind != domain.ActionKindTransfer || command.action.Card != card {
		t.Fatalf("action = %+v, want transfer with %v", command.action, card)
	}
}

func TestParseCommandParsesFinishTake(t *testing.T) {
	action := domain.Action{Kind: domain.ActionKindFinishTake, Seat: defaultHumanSeat}
	decision := app.DecisionContext{LegalActions: []domain.Action{action}}

	command, err := parseCommand("done", &decision)
	if err != nil {
		t.Fatalf("parseCommand returned error: %v", err)
	}
	if command.action != action {
		t.Fatalf("action = %+v, want finish take", command.action)
	}
}

func TestParseCommandParsesConcede(t *testing.T) {
	command, err := parseCommand("surrender", &app.DecisionContext{})
	if err != nil {
		t.Fatalf("parseCommand returned error: %v", err)
	}
	if command.kind != commandConcede {
		t.Fatalf("kind = %v, want commandConcede", command.kind)
	}
}

func TestParseCommandRejectsIllegalAction(t *testing.T) {
	card := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	decision := app.DecisionContext{Hand: []domain.Card{card}}

	if _, err := parseCommand("a 1", &decision); err == nil {
		t.Fatal("parseCommand returned nil error, want illegal action error")
	}
}
