package domain

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
	ThrowInsFromAllPlayers            bool
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
		ThrowInsFromAllPlayers:            true,
		FirstSuccessfulDefenseAttackLimit: 5,
		MaxSetupAttempts:                  1000,
	}
}
