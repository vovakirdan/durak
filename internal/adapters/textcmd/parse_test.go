package textcmd_test

import (
	"testing"

	"github.com/vovakirdan/durak/internal/adapters/textcmd"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestParseSelectsNumberedLegalAction(t *testing.T) {
	second := domain.Action{
		Kind: domain.ActionKindAttack,
		Seat: domain.Seat(0),
		Card: domain.Card{Rank: domain.Seven, Suit: domain.Clubs},
	}
	decision := app.DecisionContext{
		LegalActions: []domain.Action{
			{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: domain.Card{Rank: domain.Six, Suit: domain.Clubs}},
			second,
		},
	}

	command, err := textcmd.Parse("2", &decision)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if command.Kind != textcmd.KindAction || command.Action != second {
		t.Fatalf("command = %+v, want second legal action", command)
	}
}

func TestParseParsesAttackByHandIndex(t *testing.T) {
	card := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	decision := app.DecisionContext{
		Hand: []domain.Card{card},
		LegalActions: []domain.Action{
			{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: card},
		},
	}

	command, err := textcmd.Parse("a 1", &decision)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if command.Action.Kind != domain.ActionKindAttack || command.Action.Card != card {
		t.Fatalf("action = %+v, want attack with %v", command.Action, card)
	}
}

func TestParseParsesAttackPacket(t *testing.T) {
	first := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	second := domain.Card{Rank: domain.Six, Suit: domain.Diamonds}
	packet := domain.NewAttackAction(domain.Seat(0), first, second)
	decision := app.DecisionContext{
		Hand:         []domain.Card{first, second},
		LegalActions: []domain.Action{packet},
	}

	command, err := textcmd.Parse("attack 6D 6C", &decision)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if command.Action != packet {
		t.Fatalf("action = %+v, want packet %+v", command.Action, packet)
	}
	if got := textcmd.FormatActionCommand(packet); got != "attack 6C 6D" {
		t.Fatalf("FormatActionCommand = %q, want packet command", got)
	}
}

func TestParseParsesDefendByAttackNumberAndCardCode(t *testing.T) {
	card := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	decision := app.DecisionContext{
		Hand: []domain.Card{card},
		LegalActions: []domain.Action{
			{Kind: domain.ActionKindDefend, Seat: domain.Seat(0), Card: card, AttackIndex: 1},
		},
	}

	command, err := textcmd.Parse("defend 2 7c", &decision)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if command.Action.Kind != domain.ActionKindDefend || command.Action.AttackIndex != 1 || command.Action.Card != card {
		t.Fatalf("action = %+v, want defend second attack", command.Action)
	}
}

func TestParseParsesTransferByCardCode(t *testing.T) {
	card := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	decision := app.DecisionContext{
		Hand: []domain.Card{card},
		LegalActions: []domain.Action{
			{Kind: domain.ActionKindTransfer, Seat: domain.Seat(0), Card: card},
		},
	}

	command, err := textcmd.Parse("tr 7c", &decision)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if command.Action.Kind != domain.ActionKindTransfer || command.Action.Card != card {
		t.Fatalf("action = %+v, want transfer with %v", command.Action, card)
	}
}

func TestParseParsesFinishTake(t *testing.T) {
	action := domain.Action{Kind: domain.ActionKindFinishTake, Seat: domain.Seat(0)}
	decision := app.DecisionContext{LegalActions: []domain.Action{action}}

	command, err := textcmd.Parse("done", &decision)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if command.Action != action {
		t.Fatalf("action = %+v, want finish take", command.Action)
	}
}

func TestParseParsesConcede(t *testing.T) {
	command, err := textcmd.Parse("surrender", &app.DecisionContext{})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if command.Kind != textcmd.KindConcede {
		t.Fatalf("kind = %v, want KindConcede", command.Kind)
	}
}

func TestParseRejectsIllegalAction(t *testing.T) {
	card := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	decision := app.DecisionContext{Hand: []domain.Card{card}}

	if _, err := textcmd.Parse("a 1", &decision); err == nil {
		t.Fatal("Parse returned nil error, want illegal action error")
	}
}

func TestFormatActionCommandRoundTripsDefend(t *testing.T) {
	card := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	action := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(0),
		Card:        card,
		AttackIndex: 1,
	}
	decision := app.DecisionContext{
		Hand:         []domain.Card{card},
		LegalActions: []domain.Action{action},
	}

	command, err := textcmd.Parse(textcmd.FormatActionCommand(action), &decision)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if command.Action != action {
		t.Fatalf("action = %+v, want %+v", command.Action, action)
	}
}
