package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/vovakirdan/durak/internal/domain"
)

const (
	// CurrentMatchConfigSchemaVersion identifies the current in-process config model.
	CurrentMatchConfigSchemaVersion = 1
	// RulePresetDefault is the built-in house-rule preset from the PRD.
	RulePresetDefault = "default"
)

var (
	// ErrUnknownRulePreset means a named rule preset is not registered.
	ErrUnknownRulePreset = errors.New("unknown rules preset")
	// ErrInvalidMatchConfig means a match config is internally inconsistent.
	ErrInvalidMatchConfig = errors.New("invalid match config")
	// ErrUnsupportedMatchConfig means a future config option was selected before implementation.
	ErrUnsupportedMatchConfig = errors.New("unsupported match config")
)

// MatchConfig is the app-level value object used to create a match.
type MatchConfig struct {
	SchemaVersion int
	RulePreset    string
	Seats         SeatConfig
	Series        SeriesConfig
	Rules         RuleConfig
}

// SeatConfig describes table occupancy for one match.
type SeatConfig struct {
	PlayerCount int
}

// SeriesConfig describes how consecutive matches are linked.
type SeriesConfig struct {
	Consecutive         bool
	FirstAttackerPolicy SeriesFirstAttackerPolicy
}

// SeriesFirstAttackerPolicy controls the first attacker after a completed match.
type SeriesFirstAttackerPolicy uint8

const (
	// SeriesFirstAttackerBeforePreviousLoser starts before the last loser.
	SeriesFirstAttackerBeforePreviousLoser SeriesFirstAttackerPolicy = iota + 1
)

// TurnDirection controls table order.
type TurnDirection uint8

const (
	// TurnDirectionClockwise is the currently implemented table direction.
	TurnDirectionClockwise TurnDirection = iota + 1
	// TurnDirectionCounterClockwise is reserved for future variants.
	TurnDirectionCounterClockwise
)

// DeckLayout identifies supported deck layouts.
type DeckLayout uint8

const (
	// DeckLayout36 is the classic 36-card Durak deck.
	DeckLayout36 DeckLayout = iota + 1
)

// TrumpIndicatorPolicy controls where the visible trump card is selected from.
type TrumpIndicatorPolicy uint8

const (
	// TrumpIndicatorStockBottom selects the bottom card of the undealt stock.
	TrumpIndicatorStockBottom TrumpIndicatorPolicy = iota + 1
)

// ForbiddenTrumpPolicy controls what happens when the trump card is forbidden.
type ForbiddenTrumpPolicy uint8

const (
	// ForbiddenTrumpReselectStock reshuffles only stock and chooses a new trump card.
	ForbiddenTrumpReselectStock ForbiddenTrumpPolicy = iota + 1
)

// FirstAttackerPolicy controls first attacker selection for an independent match.
type FirstAttackerPolicy uint8

const (
	// FirstAttackerLowestTrump picks the seat holding the lowest trump.
	FirstAttackerLowestTrump FirstAttackerPolicy = iota + 1
)

// FirstAttackerFallback controls first attacker selection when nobody has trump.
type FirstAttackerFallback uint8

const (
	// FirstAttackerFallbackRandom picks a random seat.
	FirstAttackerFallbackRandom FirstAttackerFallback = iota + 1
)

// TransferRankPolicy controls which card rank can transfer an attack.
type TransferRankPolicy uint8

const (
	// TransferSameRankOnTable requires a rank already present on the table.
	TransferSameRankOnTable TransferRankPolicy = iota + 1
)

// TransferTargetPolicy controls who receives a transferred attack.
type TransferTargetPolicy uint8

const (
	// TransferToNextActiveSeat transfers to the next active seat in table order.
	TransferToNextActiveSeat TransferTargetPolicy = iota + 1
)

// RefillOrder controls card draw order after a round.
type RefillOrder uint8

const (
	// RefillAttackParticipantsThenDefender draws for attackers first, defender last.
	RefillAttackParticipantsThenDefender RefillOrder = iota + 1
)

// TimeoutPolicy controls automatic behavior after a move timeout.
type TimeoutPolicy uint8

const (
	// TimeoutPolicyNone means move timers are not active.
	TimeoutPolicyNone TimeoutPolicy = iota + 1
	// TimeoutPolicyAutoPass is reserved for future timed matches.
	TimeoutPolicyAutoPass
	// TimeoutPolicyAutoTake is reserved for future timed matches.
	TimeoutPolicyAutoTake
	// TimeoutPolicyAutoConcede is reserved for future timed matches.
	TimeoutPolicyAutoConcede
)

// RuleConfig describes configurable game rules before they become a domain profile.
type RuleConfig struct {
	Name          string
	Table         TableConfig
	Deck          DeckConfig
	Deal          DealConfig
	FirstAttacker FirstAttackerConfig
	ThrowIn       ThrowInConfig
	Defense       DefenseConfig
	Transfer      TransferConfig
	Refill        RefillConfig
	Completion    CompletionConfig
	Timing        TimingConfig
	Cheating      CheatingConfig
}

// TableConfig describes table-level rule constraints.
type TableConfig struct {
	MaxPlayers    int
	TurnDirection TurnDirection
}

// DeckConfig describes the card deck used by a match.
type DeckConfig struct {
	Layout  DeckLayout
	MinRank domain.Rank
}

// DealConfig describes initial hand, redeal, and trump setup rules.
type DealConfig struct {
	InitialHandSize             int
	RedealSameSuitThreshold     int
	TrumpIndicatorPolicy        TrumpIndicatorPolicy
	TrumpIndicatorForbiddenRank domain.Rank
	ForbiddenTrumpPolicy        ForbiddenTrumpPolicy
	MaxSetupAttempts            int
}

// FirstAttackerConfig describes independent-match first attacker selection.
type FirstAttackerConfig struct {
	Policy          FirstAttackerPolicy
	NoTrumpFallback FirstAttackerFallback
}

// ThrowInConfig describes who can add cards and how throw-in windows close.
type ThrowInConfig struct {
	Enabled                           bool
	PlayerScope                       domain.ThrowInPlayerScope
	Timing                            domain.ThrowInTiming
	Opening                           domain.ThrowInOpening
	Close                             domain.ThrowInClose
	AttackLimitPolicy                 domain.AttackLimitPolicy
	FirstSuccessfulDefenseAttackLimit int
	AllowAfterTake                    bool
}

// DefenseConfig describes defender options.
type DefenseConfig struct {
	TakeAllowed             bool
	TakeAfterPartialDefense bool
}

// TransferConfig describes transfer availability and target rules.
type TransferConfig struct {
	Enabled             bool
	FirstAttackAllowed  bool
	AfterDefenseAllowed bool
	RankPolicy          TransferRankPolicy
	TargetPolicy        TransferTargetPolicy
}

// RefillConfig describes draw order after a round.
type RefillConfig struct {
	Order RefillOrder
}

// CompletionConfig describes match-ending variants.
type CompletionConfig struct {
	DrawAllowed bool
}

// TimingConfig describes move timers and timeout behavior.
type TimingConfig struct {
	MoveTimeout    time.Duration
	ThrowInTimeout time.Duration
	TimeoutPolicy  TimeoutPolicy
}

// CheatingConfig reserves future cheating-rule variants.
type CheatingConfig struct {
	Enabled bool
}

// NewMatchConfig creates a validated match config from a built-in rule preset.
func NewMatchConfig(rulePreset string, playerCount int) (MatchConfig, error) {
	if rulePreset == "" {
		rulePreset = RulePresetDefault
	}
	rules, err := RuleConfigPreset(rulePreset)
	if err != nil {
		return MatchConfig{}, err
	}
	config := MatchConfig{
		SchemaVersion: CurrentMatchConfigSchemaVersion,
		RulePreset:    rulePreset,
		Seats:         SeatConfig{PlayerCount: playerCount},
		Series: SeriesConfig{
			Consecutive:         true,
			FirstAttackerPolicy: SeriesFirstAttackerBeforePreviousLoser,
		},
		Rules: rules,
	}
	err = config.Validate()
	return config, err
}

// RuleConfigPreset returns a built-in rules config by name.
func RuleConfigPreset(name string) (RuleConfig, error) {
	if name == "" {
		name = RulePresetDefault
	}
	switch name {
	case RulePresetDefault:
		return DefaultRuleConfig(), nil
	default:
		return RuleConfig{}, fmt.Errorf("%w %q", ErrUnknownRulePreset, name)
	}
}

// RuleProfilePreset returns a domain profile from a built-in rules config.
func RuleProfilePreset(name string) (domain.RuleProfile, error) {
	rules, err := RuleConfigPreset(name)
	if err != nil {
		return domain.RuleProfile{}, err
	}
	return rules.RuleProfile()
}

// RuleProfile maps a validated match config into the domain rule profile.
func (c *MatchConfig) RuleProfile() (domain.RuleProfile, error) {
	if err := c.Validate(); err != nil {
		return domain.RuleProfile{}, err
	}
	return (&c.Rules).RuleProfile()
}

// RuleProfile maps app-level rule config into the domain state-machine profile.
func (c *RuleConfig) RuleProfile() (domain.RuleProfile, error) {
	if err := c.Validate(); err != nil {
		return domain.RuleProfile{}, err
	}
	return domain.RuleProfile{
		Name:                              c.Name,
		DeckMinRank:                       c.Deck.MinRank,
		InitialHandSize:                   c.Deal.InitialHandSize,
		MaxPlayers:                        c.Table.MaxPlayers,
		RedealSameSuitThreshold:           c.Deal.RedealSameSuitThreshold,
		TrumpIndicatorForbiddenRank:       c.Deal.TrumpIndicatorForbiddenRank,
		TransferEnabled:                   c.Transfer.Enabled,
		FirstAttackTransferAllowed:        c.Transfer.FirstAttackAllowed,
		ThrowInPlayerScope:                c.ThrowIn.PlayerScope,
		ThrowInTiming:                     c.ThrowIn.Timing,
		ThrowInOpening:                    c.ThrowIn.Opening,
		ThrowInClose:                      c.ThrowIn.Close,
		AttackLimitPolicy:                 c.ThrowIn.AttackLimitPolicy,
		FirstSuccessfulDefenseAttackLimit: c.ThrowIn.FirstSuccessfulDefenseAttackLimit,
		MaxSetupAttempts:                  c.Deal.MaxSetupAttempts,
	}, nil
}
