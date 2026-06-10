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
	if !profile.ThrowInsFromAllPlayers {
		t.Fatal("ThrowInsFromAllPlayers = false, want true")
	}
	if profile.FirstSuccessfulDefenseAttackLimit != 5 {
		t.Fatalf("FirstSuccessfulDefenseAttackLimit = %d, want 5", profile.FirstSuccessfulDefenseAttackLimit)
	}
}
