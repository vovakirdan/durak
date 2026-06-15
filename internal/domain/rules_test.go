package domain_test

import (
	"testing"

	. "github.com/vovakirdan/durak/internal/domain"
)

func TestDefaultRuleProfile(t *testing.T) {
	profile := DefaultRuleProfile()

	if profile.InitialHandSize != 6 {
		t.Fatalf("InitialHandSize = %d, want 6", profile.InitialHandSize)
	}
	if profile.MaxPlayers != 6 {
		t.Fatalf("MaxPlayers = %d, want 6", profile.MaxPlayers)
	}
	if profile.RedealSameSuitThreshold != 5 {
		t.Fatalf("RedealSameSuitThreshold = %d, want 5", profile.RedealSameSuitThreshold)
	}
	if profile.TrumpIndicatorForbiddenRank != Ace {
		t.Fatalf("TrumpIndicatorForbiddenRank = %v, want %v", profile.TrumpIndicatorForbiddenRank, Ace)
	}
	if !profile.TransferEnabled {
		t.Fatal("TransferEnabled = false, want true")
	}
	if profile.FirstAttackTransferAllowed {
		t.Fatal("FirstAttackTransferAllowed = true, want false")
	}
	if profile.ThrowInPlayerScope != ThrowInPlayerScopeAllExceptDefender {
		t.Fatalf("ThrowInPlayerScope = %v, want %v", profile.ThrowInPlayerScope, ThrowInPlayerScopeAllExceptDefender)
	}
	if profile.ThrowInTiming != ThrowInTimingAnyEligible {
		t.Fatalf("ThrowInTiming = %v, want %v", profile.ThrowInTiming, ThrowInTimingAnyEligible)
	}
	if profile.ThrowInOpening != ThrowInOpeningLeadFirst {
		t.Fatalf("ThrowInOpening = %v, want %v", profile.ThrowInOpening, ThrowInOpeningLeadFirst)
	}
	if profile.ThrowInClose != ThrowInCloseAllEligiblePassed {
		t.Fatalf("ThrowInClose = %v, want %v", profile.ThrowInClose, ThrowInCloseAllEligiblePassed)
	}
	if profile.AttackLimitPolicy != AttackLimitUnlimited {
		t.Fatalf("AttackLimitPolicy = %v, want %v", profile.AttackLimitPolicy, AttackLimitUnlimited)
	}
	if profile.FirstSuccessfulDefenseAttackLimit != 5 {
		t.Fatalf("FirstSuccessfulDefenseAttackLimit = %d, want 5", profile.FirstSuccessfulDefenseAttackLimit)
	}
}
