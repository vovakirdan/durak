package main

import (
	"github.com/vovakirdan/durak/internal/app"
)

const defaultRulesPreset = app.RulePresetDefault

func matchConfig(name string, playerCount int) (app.MatchConfig, error) {
	return app.NewMatchConfig(name, playerCount)
}
