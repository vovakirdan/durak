package main

import (
	"fmt"

	"github.com/vovakirdan/durak/internal/domain"
)

const defaultRulesPreset = "default"

func ruleProfile(name string) (domain.RuleProfile, error) {
	if name == "" {
		name = defaultRulesPreset
	}
	switch name {
	case defaultRulesPreset:
		return domain.DefaultRuleProfile(), nil
	default:
		return domain.RuleProfile{}, fmt.Errorf("unknown rules preset %q", name)
	}
}
