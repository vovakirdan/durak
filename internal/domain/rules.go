package domain

// ThrowInPlayerScope controls which seats may add cards after the first attack.
type ThrowInPlayerScope uint8

const (
	// ThrowInPlayerScopeLeadOnly allows only the current lead attacker.
	ThrowInPlayerScopeLeadOnly ThrowInPlayerScope = iota + 1
	// ThrowInPlayerScopeNeighborsOnly allows the lead attacker and defender neighbors.
	ThrowInPlayerScopeNeighborsOnly
	// ThrowInPlayerScopeAllExceptDefender allows every active seat except the defender.
	ThrowInPlayerScopeAllExceptDefender
)

// ThrowInTiming controls whether eligible throw-ins are free-form or ordered.
type ThrowInTiming uint8

const (
	// ThrowInTimingAnyEligible accepts a legal throw-in from any eligible seat.
	ThrowInTimingAnyEligible ThrowInTiming = iota + 1
	// ThrowInTimingClockwise is reserved for ordered throw-in variants.
	ThrowInTimingClockwise
)

// ThrowInOpening controls who gets the first optional throw-in opportunity.
type ThrowInOpening uint8

const (
	// ThrowInOpeningAnyEligible lets any eligible seat add the first throw-in.
	ThrowInOpeningAnyEligible ThrowInOpening = iota + 1
	// ThrowInOpeningLeadFirst gives the lead attacker the first opportunity.
	ThrowInOpeningLeadFirst
)

// ThrowInClose controls when a round may close after optional throw-ins.
type ThrowInClose uint8

const (
	// ThrowInCloseLeadMayClose allows the lead attacker to finish immediately.
	ThrowInCloseLeadMayClose ThrowInClose = iota + 1
	// ThrowInCloseAllEligiblePassed requires other throw-capable seats to pass.
	ThrowInCloseAllEligiblePassed
)

// AttackLimitPolicy controls contextual limits on attack-card count.
type AttackLimitPolicy uint8

const (
	// AttackLimitUnlimited allows throw-ins beyond the defender's starting hand size.
	AttackLimitUnlimited AttackLimitPolicy = iota + 1
	// AttackLimitByDefenderInitialHand caps attacks by the defender's round-start hand size.
	AttackLimitByDefenderInitialHand
)

// RuleProfile describes configurable match rules.
type RuleProfile struct {
	Name                              string
	DeckMinRank                       Rank
	InitialHandSize                   int
	MaxPlayers                        int
	RedealSameSuitThreshold           int
	TrumpIndicatorForbiddenRank       Rank
	TransferEnabled                   bool
	FirstAttackTransferAllowed        bool
	ThrowInPlayerScope                ThrowInPlayerScope
	ThrowInTiming                     ThrowInTiming
	ThrowInOpening                    ThrowInOpening
	ThrowInClose                      ThrowInClose
	AttackLimitPolicy                 AttackLimitPolicy
	FirstSuccessfulDefenseAttackLimit int
	MaxSetupAttempts                  int
}

// DefaultRuleProfile returns the initial house-rule preset from the PRD.
func DefaultRuleProfile() RuleProfile {
	return RuleProfile{
		Name:                              "default",
		DeckMinRank:                       Six,
		InitialHandSize:                   6,
		MaxPlayers:                        6,
		RedealSameSuitThreshold:           5,
		TrumpIndicatorForbiddenRank:       Ace,
		TransferEnabled:                   true,
		FirstAttackTransferAllowed:        false,
		ThrowInPlayerScope:                ThrowInPlayerScopeAllExceptDefender,
		ThrowInTiming:                     ThrowInTimingAnyEligible,
		ThrowInOpening:                    ThrowInOpeningLeadFirst,
		ThrowInClose:                      ThrowInCloseAllEligiblePassed,
		AttackLimitPolicy:                 AttackLimitUnlimited,
		FirstSuccessfulDefenseAttackLimit: 5,
		MaxSetupAttempts:                  1000,
	}
}
