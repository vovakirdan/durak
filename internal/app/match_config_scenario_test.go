package app_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

var configScenarioTrump = domain.Card{Rank: domain.Nine, Suit: domain.Hearts}

func TestMatchConfigTransferRulesDriveMatch(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	transfer := domain.Card{Rank: domain.Six, Suit: domain.Diamonds}

	t.Run("first attack transfer allowed", func(t *testing.T) {
		scenario := newConfiguredMatchScenario(t, [][]domain.Card{
			{attack},
			{transfer},
		}, func(config *app.MatchConfig) {
			config.Rules.Transfer.FirstAttackAllowed = true
		})

		scenario.attack(attack)
		action := domain.Action{Kind: domain.ActionKindTransfer, Seat: domain.Seat(1), Card: transfer}
		scenario.expectLegal(action)
		scenario.apply(action)
		scenario.expectRoles(domain.Seat(1), domain.Seat(0))
	})

	t.Run("transfer disabled", func(t *testing.T) {
		scenario := newConfiguredMatchScenario(t, [][]domain.Card{
			{attack},
			{transfer},
		}, func(config *app.MatchConfig) {
			config.Rules.Transfer.Enabled = false
			config.Rules.Transfer.FirstAttackAllowed = true
		})

		scenario.attack(attack)
		action := domain.Action{Kind: domain.ActionKindTransfer, Seat: domain.Seat(1), Card: transfer}
		scenario.expectNotLegal(action)
		scenario.reject(action, domain.ErrTransferDisabled)
	})
}

func TestMatchConfigThrowInScopeRulesDriveMatch(t *testing.T) {
	tests := []struct {
		name  string
		scope domain.ThrowInPlayerScope
		want  map[domain.Seat]bool
	}{
		{
			name:  "lead only",
			scope: domain.ThrowInPlayerScopeLeadOnly,
			want: map[domain.Seat]bool{
				0: true,
				2: false,
				3: false,
			},
		},
		{
			name:  "neighbors only",
			scope: domain.ThrowInPlayerScopeNeighborsOnly,
			want: map[domain.Seat]bool{
				0: true,
				2: true,
				3: false,
			},
		},
		{
			name:  "all except defender",
			scope: domain.ThrowInPlayerScopeAllExceptDefender,
			want: map[domain.Seat]bool{
				0: true,
				2: true,
				3: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenario := newConfiguredMatchScenario(t, throwInScopeHands(), func(config *app.MatchConfig) {
				config.Rules.ThrowIn.PlayerScope = tt.scope
				config.Rules.ThrowIn.Opening = domain.ThrowInOpeningAnyEligible
			})
			scenario.openDefendedThrowIn()

			actions := map[domain.Seat]domain.Action{
				0: {Kind: domain.ActionKindThrowIn, Seat: 0, Card: domain.Card{Rank: domain.Six, Suit: domain.Diamonds}},
				2: {Kind: domain.ActionKindThrowIn, Seat: 2, Card: domain.Card{Rank: domain.Six, Suit: domain.Hearts}},
				3: {Kind: domain.ActionKindThrowIn, Seat: 3, Card: domain.Card{Rank: domain.Six, Suit: domain.Spades}},
			}
			for seat, action := range actions {
				if tt.want[seat] {
					scenario.expectLegal(action)
				} else {
					scenario.expectNotLegal(action)
				}
			}
		})
	}
}

func TestMatchConfigThrowInCloseRulesDriveMatch(t *testing.T) {
	tests := []struct {
		name      string
		closeRule domain.ThrowInClose
		wantLegal bool
	}{
		{
			name:      "lead may close immediately",
			closeRule: domain.ThrowInCloseLeadMayClose,
			wantLegal: true,
		},
		{
			name:      "all eligible must pass",
			closeRule: domain.ThrowInCloseAllEligiblePassed,
			wantLegal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenario := newConfiguredMatchScenario(t, [][]domain.Card{
				{{Rank: domain.Six, Suit: domain.Clubs}},
				{{Rank: domain.Seven, Suit: domain.Clubs}},
				{{Rank: domain.Six, Suit: domain.Diamonds}},
			}, func(config *app.MatchConfig) {
				config.Rules.ThrowIn.Close = tt.closeRule
			})
			scenario.openDefendedThrowIn()

			action := domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)}
			if tt.wantLegal {
				scenario.expectLegal(action)
			} else {
				scenario.expectNotLegal(action)
			}
		})
	}
}

func TestMatchConfigDefenderInitialHandAttackLimitDrivesMatch(t *testing.T) {
	scenario := newConfiguredMatchScenario(t, [][]domain.Card{
		{
			{Rank: domain.Six, Suit: domain.Clubs},
			{Rank: domain.Six, Suit: domain.Diamonds},
			{Rank: domain.Six, Suit: domain.Hearts},
		},
		{
			{Rank: domain.Seven, Suit: domain.Clubs},
			{Rank: domain.Seven, Suit: domain.Diamonds},
		},
	}, func(config *app.MatchConfig) {
		config.Rules.ThrowIn.AttackLimitPolicy = domain.AttackLimitByDefenderInitialHand
		config.Rules.ThrowIn.FirstSuccessfulDefenseAttackLimit = 0
	})

	scenario.attack(domain.Card{Rank: domain.Six, Suit: domain.Clubs})
	scenario.defend(0, domain.Card{Rank: domain.Seven, Suit: domain.Clubs})
	scenario.throwIn(domain.Seat(0), domain.Card{Rank: domain.Six, Suit: domain.Diamonds})
	scenario.defend(1, domain.Card{Rank: domain.Seven, Suit: domain.Diamonds})

	extra := domain.Action{
		Kind: domain.ActionKindThrowIn,
		Seat: domain.Seat(0),
		Card: domain.Card{Rank: domain.Six, Suit: domain.Hearts},
	}
	scenario.expectNotLegal(extra)
	scenario.reject(extra, domain.ErrAttackLimitReached)
}

func TestMatchConfigFirstSuccessfulDefenseLimitDrivesMatch(t *testing.T) {
	scenario := newConfiguredMatchScenario(t, [][]domain.Card{
		{
			{Rank: domain.Six, Suit: domain.Clubs},
			{Rank: domain.Six, Suit: domain.Diamonds},
			{Rank: domain.Six, Suit: domain.Hearts},
		},
		{
			{Rank: domain.Seven, Suit: domain.Clubs},
			{Rank: domain.Seven, Suit: domain.Diamonds},
			{Rank: domain.Seven, Suit: domain.Hearts},
		},
	}, func(config *app.MatchConfig) {
		config.Rules.ThrowIn.FirstSuccessfulDefenseAttackLimit = 2
	})

	scenario.attack(domain.Card{Rank: domain.Six, Suit: domain.Clubs})
	scenario.defend(0, domain.Card{Rank: domain.Seven, Suit: domain.Clubs})
	scenario.throwIn(domain.Seat(0), domain.Card{Rank: domain.Six, Suit: domain.Diamonds})
	scenario.defend(1, domain.Card{Rank: domain.Seven, Suit: domain.Diamonds})

	extra := domain.Action{
		Kind: domain.ActionKindThrowIn,
		Seat: domain.Seat(0),
		Card: domain.Card{Rank: domain.Six, Suit: domain.Hearts},
	}
	scenario.expectNotLegal(extra)
	scenario.reject(extra, domain.ErrAttackLimitReached)
}

type configuredMatchScenario struct {
	t     *testing.T
	match *domain.Match
}

func newConfiguredMatchScenario(
	t *testing.T,
	hands [][]domain.Card,
	configure func(*app.MatchConfig),
) *configuredMatchScenario {
	t.Helper()
	config, err := app.NewMatchConfig(app.RulePresetDefault, len(hands))
	if err != nil {
		t.Fatalf("NewMatchConfig returned error: %v", err)
	}
	if configure != nil {
		configure(&config)
	}
	profile, err := config.RuleProfile()
	if err != nil {
		t.Fatalf("RuleProfile returned error: %v", err)
	}
	match, err := domain.NewMatch(&domain.InitialDeal{
		Hands:          hands,
		TrumpIndicator: configScenarioTrump,
		TrumpSuit:      configScenarioTrump.Suit,
		FirstAttacker:  0,
	}, profile)
	if err != nil {
		t.Fatalf("NewMatch returned error: %v", err)
	}
	return &configuredMatchScenario{t: t, match: match}
}

func throwInScopeHands() [][]domain.Card {
	return [][]domain.Card{
		{
			{Rank: domain.Six, Suit: domain.Clubs},
			{Rank: domain.Six, Suit: domain.Diamonds},
		},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
		{{Rank: domain.Six, Suit: domain.Hearts}},
		{{Rank: domain.Six, Suit: domain.Spades}},
	}
}

func (s *configuredMatchScenario) openDefendedThrowIn() {
	s.t.Helper()
	s.attack(domain.Card{Rank: domain.Six, Suit: domain.Clubs})
	s.defend(0, domain.Card{Rank: domain.Seven, Suit: domain.Clubs})
}

func (s *configuredMatchScenario) attack(card domain.Card) {
	s.t.Helper()
	s.apply(domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: card})
}

func (s *configuredMatchScenario) defend(attackIndex int, card domain.Card) {
	s.t.Helper()
	s.apply(domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		AttackIndex: attackIndex,
		Card:        card,
	})
}

func (s *configuredMatchScenario) throwIn(seat domain.Seat, card domain.Card) {
	s.t.Helper()
	s.apply(domain.Action{Kind: domain.ActionKindThrowIn, Seat: seat, Card: card})
}

func (s *configuredMatchScenario) apply(action domain.Action) {
	s.t.Helper()
	if !slices.Contains(s.match.LegalActions(action.Seat), action) {
		s.t.Fatalf("action %+v is not legal; legal actions: %+v", action, s.match.LegalActions(action.Seat))
	}
	if err := s.match.ApplyAction(action); err != nil {
		s.t.Fatalf("ApplyAction(%+v) returned error: %v", action, err)
	}
}

func (s *configuredMatchScenario) reject(action domain.Action, want error) {
	s.t.Helper()
	err := s.match.ApplyAction(action)
	if !errors.Is(err, want) {
		s.t.Fatalf("ApplyAction(%+v) error = %v, want %v", action, err, want)
	}
}

func (s *configuredMatchScenario) expectLegal(action domain.Action) {
	s.t.Helper()
	if !slices.Contains(s.match.LegalActions(action.Seat), action) {
		s.t.Fatalf("action %+v is not legal; legal actions: %+v", action, s.match.LegalActions(action.Seat))
	}
}

func (s *configuredMatchScenario) expectNotLegal(action domain.Action) {
	s.t.Helper()
	if slices.Contains(s.match.LegalActions(action.Seat), action) {
		s.t.Fatalf("action %+v is legal, want it rejected", action)
	}
}

func (s *configuredMatchScenario) expectRoles(attacker, defender domain.Seat) {
	s.t.Helper()
	if s.match.Attacker() != attacker || s.match.Defender() != defender {
		s.t.Fatalf("roles = attacker %d defender %d, want %d/%d",
			s.match.Attacker(), s.match.Defender(), attacker, defender)
	}
}
