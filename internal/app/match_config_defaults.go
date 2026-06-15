package app

import "github.com/vovakirdan/durak/internal/domain"

// DefaultRuleConfig returns the built-in house-rule config from the PRD.
func DefaultRuleConfig() RuleConfig {
	profile := domain.DefaultRuleProfile()
	return RuleConfig{
		Name: profile.Name,
		Table: TableConfig{
			MaxPlayers:    profile.MaxPlayers,
			TurnDirection: TurnDirectionClockwise,
		},
		Deck: DeckConfig{
			Layout:  DeckLayout36,
			MinRank: profile.DeckMinRank,
		},
		Deal: DealConfig{
			InitialHandSize:             profile.InitialHandSize,
			RedealSameSuitThreshold:     profile.RedealSameSuitThreshold,
			TrumpIndicatorPolicy:        TrumpIndicatorStockBottom,
			TrumpIndicatorForbiddenRank: profile.TrumpIndicatorForbiddenRank,
			ForbiddenTrumpPolicy:        ForbiddenTrumpReselectStock,
			MaxSetupAttempts:            profile.MaxSetupAttempts,
		},
		FirstAttacker: FirstAttackerConfig{
			Policy:          FirstAttackerLowestTrump,
			NoTrumpFallback: FirstAttackerFallbackRandom,
		},
		ThrowIn: ThrowInConfig{
			Enabled:                           true,
			PlayerScope:                       profile.ThrowInPlayerScope,
			Timing:                            profile.ThrowInTiming,
			Opening:                           profile.ThrowInOpening,
			Close:                             profile.ThrowInClose,
			AttackLimitPolicy:                 profile.AttackLimitPolicy,
			FirstSuccessfulDefenseAttackLimit: profile.FirstSuccessfulDefenseAttackLimit,
			AllowAfterTake:                    true,
		},
		Defense: DefenseConfig{
			TakeAllowed:             true,
			TakeAfterPartialDefense: true,
		},
		Transfer: TransferConfig{
			Enabled:             profile.TransferEnabled,
			FirstAttackAllowed:  profile.FirstAttackTransferAllowed,
			AfterDefenseAllowed: false,
			RankPolicy:          TransferSameRankOnTable,
			TargetPolicy:        TransferToNextActiveSeat,
		},
		Refill: RefillConfig{
			Order: RefillAttackParticipantsThenDefender,
		},
		Completion: CompletionConfig{
			DrawAllowed: true,
		},
		Timing: TimingConfig{
			TimeoutPolicy: TimeoutPolicyNone,
		},
	}
}
