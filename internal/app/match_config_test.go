package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestDefaultMatchConfigMapsToDefaultRuleProfile(t *testing.T) {
	config, err := app.NewMatchConfig(app.RulePresetDefault, 4)
	if err != nil {
		t.Fatalf("NewMatchConfig returned error: %v", err)
	}

	profile, err := config.Rules.RuleProfile()
	if err != nil {
		t.Fatalf("RuleProfile returned error: %v", err)
	}
	want := domain.DefaultRuleProfile()
	if profile != want {
		t.Fatalf("RuleProfile = %+v, want %+v", profile, want)
	}
	if config.Seats.PlayerCount != 4 {
		t.Fatalf("PlayerCount = %d, want 4", config.Seats.PlayerCount)
	}
	if config.SchemaVersion != app.CurrentMatchConfigSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", config.SchemaVersion, app.CurrentMatchConfigSchemaVersion)
	}
}

func TestRuleProfilePresetRejectsUnknownPreset(t *testing.T) {
	_, err := app.RuleProfilePreset("custom")
	if err == nil {
		t.Fatal("RuleProfilePreset returned nil error, want unknown preset")
	}
	if !errors.Is(err, app.ErrUnknownRulePreset) {
		t.Fatalf("error = %v, want ErrUnknownRulePreset", err)
	}
}

func TestMatchConfigValidatesSeatCountAgainstRules(t *testing.T) {
	_, err := app.NewMatchConfig(app.RulePresetDefault, 7)
	if err == nil {
		t.Fatal("NewMatchConfig returned nil error, want invalid seats")
	}
	if !errors.Is(err, app.ErrInvalidMatchConfig) {
		t.Fatalf("error = %v, want ErrInvalidMatchConfig", err)
	}
}

func TestRuleConfigSupportsImplementedVariants(t *testing.T) {
	config := app.DefaultRuleConfig()
	config.ThrowIn.PlayerScope = domain.ThrowInPlayerScopeNeighborsOnly
	config.ThrowIn.Opening = domain.ThrowInOpeningAnyEligible
	config.ThrowIn.Close = domain.ThrowInCloseLeadMayClose
	config.ThrowIn.AttackLimitPolicy = domain.AttackLimitByDefenderInitialHand
	config.Transfer.Enabled = false
	config.Transfer.FirstAttackAllowed = true

	profile, err := config.RuleProfile()
	if err != nil {
		t.Fatalf("RuleProfile returned error: %v", err)
	}
	if profile.ThrowInPlayerScope != domain.ThrowInPlayerScopeNeighborsOnly {
		t.Fatalf("ThrowInPlayerScope = %v, want neighbors only", profile.ThrowInPlayerScope)
	}
	if profile.AttackLimitPolicy != domain.AttackLimitByDefenderInitialHand {
		t.Fatalf("AttackLimitPolicy = %v, want defender-hand limit", profile.AttackLimitPolicy)
	}
	if profile.TransferEnabled {
		t.Fatal("TransferEnabled = true, want false")
	}
}

func TestRuleConfigRejectsUnsupportedFutureOptions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*app.RuleConfig)
	}{
		{
			name: "counter clockwise turns",
			mutate: func(config *app.RuleConfig) {
				config.Table.TurnDirection = app.TurnDirectionCounterClockwise
			},
		},
		{
			name: "ordered throw-ins",
			mutate: func(config *app.RuleConfig) {
				config.ThrowIn.Timing = domain.ThrowInTimingClockwise
			},
		},
		{
			name: "timed moves",
			mutate: func(config *app.RuleConfig) {
				config.Timing.MoveTimeout = time.Second
				config.Timing.TimeoutPolicy = app.TimeoutPolicyAutoTake
			},
		},
		{
			name: "cheating rules",
			mutate: func(config *app.RuleConfig) {
				config.Cheating.Enabled = true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := app.DefaultRuleConfig()
			tt.mutate(&config)

			_, err := config.RuleProfile()
			if err == nil {
				t.Fatal("RuleProfile returned nil error, want unsupported config")
			}
			if !errors.Is(err, app.ErrUnsupportedMatchConfig) {
				t.Fatalf("error = %v, want ErrUnsupportedMatchConfig", err)
			}
		})
	}
}
