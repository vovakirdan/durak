package app

import (
	"errors"
	"fmt"

	"github.com/vovakirdan/durak/internal/domain"
)

// Validate checks that config can be represented by the current implementation.
func (c *MatchConfig) Validate() error {
	if c == nil {
		return invalidConfig("match config is nil")
	}
	if c.SchemaVersion != CurrentMatchConfigSchemaVersion {
		return invalidConfig("schema version %d", c.SchemaVersion)
	}
	profile, err := (&c.Rules).RuleProfile()
	if err != nil {
		return err
	}
	if c.Seats.PlayerCount < 2 || c.Seats.PlayerCount > profile.MaxPlayers {
		return fmt.Errorf("%w: seats must be in range 2..%d", ErrInvalidMatchConfig, profile.MaxPlayers)
	}
	if c.Series.FirstAttackerPolicy != SeriesFirstAttackerBeforePreviousLoser {
		return unsupportedConfig("series first attacker policy %d", c.Series.FirstAttackerPolicy)
	}
	return nil
}

// Validate checks that rule config can be represented by the current domain core.
func (c *RuleConfig) Validate() error {
	if c == nil {
		return invalidConfig("rule config is nil")
	}
	var errs []error
	if c.Name == "" {
		errs = append(errs, invalidConfig("rule name is empty"))
	}
	if c.Table.MaxPlayers < 2 {
		errs = append(errs, invalidConfig("max players must be at least 2"))
	}
	if c.Table.TurnDirection != TurnDirectionClockwise {
		errs = append(errs, unsupportedConfig("turn direction %d", c.Table.TurnDirection))
	}
	if c.Deck.Layout != DeckLayout36 {
		errs = append(errs, unsupportedConfig("deck layout %d", c.Deck.Layout))
	}
	if c.Deck.MinRank != domain.Six {
		errs = append(errs, unsupportedConfig("deck min rank %s", c.Deck.MinRank))
	}
	errs = append(errs, validateDealConfig(c)...)
	errs = append(errs, validateFirstAttackerConfig(c.FirstAttacker)...)
	errs = append(errs, validateThrowInConfig(c.ThrowIn)...)
	errs = append(errs, validateDefenseConfig(c.Defense)...)
	errs = append(errs, validateTransferConfig(c.Transfer)...)
	errs = append(errs, validateRemainingRuleConfig(c)...)
	return errors.Join(errs...)
}

func validateDealConfig(c *RuleConfig) []error {
	var errs []error
	if c.Deal.InitialHandSize <= 0 {
		errs = append(errs, invalidConfig("initial hand size must be positive"))
	}
	if c.Table.MaxPlayers*c.Deal.InitialHandSize > len(domain.NewDeck36()) {
		errs = append(errs, invalidConfig("max players need more than 36 cards"))
	}
	if c.Deal.RedealSameSuitThreshold < 0 {
		errs = append(errs, invalidConfig("redeal same-suit threshold cannot be negative"))
	}
	if c.Deal.TrumpIndicatorPolicy != TrumpIndicatorStockBottom {
		errs = append(errs, unsupportedConfig("trump indicator policy %d", c.Deal.TrumpIndicatorPolicy))
	}
	if c.Deal.TrumpIndicatorForbiddenRank != domain.Ace {
		errs = append(errs, unsupportedConfig("trump forbidden rank %s", c.Deal.TrumpIndicatorForbiddenRank))
	}
	if c.Deal.ForbiddenTrumpPolicy != ForbiddenTrumpReselectStock {
		errs = append(errs, unsupportedConfig("forbidden trump policy %d", c.Deal.ForbiddenTrumpPolicy))
	}
	if c.Deal.MaxSetupAttempts <= 0 {
		errs = append(errs, invalidConfig("max setup attempts must be positive"))
	}
	return errs
}

func validateFirstAttackerConfig(c FirstAttackerConfig) []error {
	var errs []error
	if c.Policy != FirstAttackerLowestTrump {
		errs = append(errs, unsupportedConfig("first attacker policy %d", c.Policy))
	}
	if c.NoTrumpFallback != FirstAttackerFallbackRandom {
		errs = append(errs, unsupportedConfig("first attacker fallback %d", c.NoTrumpFallback))
	}
	return errs
}

func validateThrowInConfig(c ThrowInConfig) []error {
	var errs []error
	if !c.Enabled {
		errs = append(errs, unsupportedConfig("disabled throw-ins"))
	}
	if !validThrowInPlayerScope(c.PlayerScope) {
		errs = append(errs, invalidConfig("throw-in player scope %d", c.PlayerScope))
	}
	if c.Timing != domain.ThrowInTimingAnyEligible {
		errs = append(errs, unsupportedConfig("throw-in timing %d", c.Timing))
	}
	if !validThrowInOpening(c.Opening) {
		errs = append(errs, invalidConfig("throw-in opening %d", c.Opening))
	}
	if !validThrowInClose(c.Close) {
		errs = append(errs, invalidConfig("throw-in close %d", c.Close))
	}
	if !validAttackLimitPolicy(c.AttackLimitPolicy) {
		errs = append(errs, invalidConfig("attack limit policy %d", c.AttackLimitPolicy))
	}
	if c.FirstSuccessfulDefenseAttackLimit < 0 {
		errs = append(errs, invalidConfig("first successful defense limit cannot be negative"))
	}
	if !c.AllowAfterTake {
		errs = append(errs, unsupportedConfig("disabled throw-ins after take"))
	}
	return errs
}

func validateDefenseConfig(c DefenseConfig) []error {
	var errs []error
	if !c.TakeAllowed {
		errs = append(errs, unsupportedConfig("disabled taking"))
	}
	if !c.TakeAfterPartialDefense {
		errs = append(errs, unsupportedConfig("disabled take after partial defense"))
	}
	return errs
}

func validateTransferConfig(c TransferConfig) []error {
	var errs []error
	if c.AfterDefenseAllowed {
		errs = append(errs, unsupportedConfig("transfer after defense"))
	}
	if c.RankPolicy != TransferSameRankOnTable {
		errs = append(errs, unsupportedConfig("transfer rank policy %d", c.RankPolicy))
	}
	if c.TargetPolicy != TransferToNextActiveSeat {
		errs = append(errs, unsupportedConfig("transfer target policy %d", c.TargetPolicy))
	}
	return errs
}

func validateRemainingRuleConfig(c *RuleConfig) []error {
	var errs []error
	if c.Refill.Order != RefillAttackParticipantsThenDefender {
		errs = append(errs, unsupportedConfig("refill order %d", c.Refill.Order))
	}
	if !c.Completion.DrawAllowed {
		errs = append(errs, unsupportedConfig("disabled draw outcome"))
	}
	if c.Timing.MoveTimeout != 0 || c.Timing.ThrowInTimeout != 0 || c.Timing.TimeoutPolicy != TimeoutPolicyNone {
		errs = append(errs, unsupportedConfig("timed match rules"))
	}
	if c.Cheating.Enabled {
		errs = append(errs, unsupportedConfig("cheating rules"))
	}
	return errs
}

func validThrowInPlayerScope(scope domain.ThrowInPlayerScope) bool {
	switch scope {
	case domain.ThrowInPlayerScopeLeadOnly,
		domain.ThrowInPlayerScopeNeighborsOnly,
		domain.ThrowInPlayerScopeAllExceptDefender:
		return true
	default:
		return false
	}
}

func validThrowInOpening(opening domain.ThrowInOpening) bool {
	switch opening {
	case domain.ThrowInOpeningAnyEligible, domain.ThrowInOpeningLeadFirst:
		return true
	default:
		return false
	}
}

func validThrowInClose(policy domain.ThrowInClose) bool {
	switch policy {
	case domain.ThrowInCloseLeadMayClose, domain.ThrowInCloseAllEligiblePassed:
		return true
	default:
		return false
	}
}

func validAttackLimitPolicy(policy domain.AttackLimitPolicy) bool {
	switch policy {
	case domain.AttackLimitUnlimited, domain.AttackLimitByDefenderInitialHand:
		return true
	default:
		return false
	}
}

func invalidConfig(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidMatchConfig, fmt.Sprintf(format, args...))
}

func unsupportedConfig(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrUnsupportedMatchConfig, fmt.Sprintf(format, args...))
}
