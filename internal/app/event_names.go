package app

import (
	"fmt"

	"github.com/vovakirdan/durak/internal/domain"
)

func eventNameForDomainKind(kind domain.EventKind) (string, bool) {
	switch kind {
	case domain.EventKindMatchStarted:
		return eventNameMatchStarted, true
	case domain.EventKindDeal:
		return eventNameDeal, true
	case domain.EventKindAttack:
		return eventNameAttack, true
	case domain.EventKindDefend:
		return eventNameDefend, true
	case domain.EventKindThrowIn:
		return eventNameThrowIn, true
	case domain.EventKindTransfer:
		return eventNameTransfer, true
	case domain.EventKindTake:
		return eventNameTake, true
	case domain.EventKindFinishDefense:
		return eventNameFinishDefense, true
	case domain.EventKindFinishTake:
		return eventNameFinishTake, true
	case domain.EventKindRefill:
		return eventNameRefill, true
	case domain.EventKindRoundEnded:
		return eventNameRoundEnded, true
	case domain.EventKindConcede:
		return eventNameConcede, true
	case domain.EventKindMatchEnded:
		return eventNameMatchEnded, true
	default:
		return "", false
	}
}

func domainKindForEventName(name string) (domain.EventKind, bool) {
	switch name {
	case eventNameMatchStarted:
		return domain.EventKindMatchStarted, true
	case eventNameDeal:
		return domain.EventKindDeal, true
	case eventNameAttack:
		return domain.EventKindAttack, true
	case eventNameDefend:
		return domain.EventKindDefend, true
	case eventNameThrowIn:
		return domain.EventKindThrowIn, true
	case eventNameTransfer:
		return domain.EventKindTransfer, true
	case eventNameTake:
		return domain.EventKindTake, true
	case eventNameFinishDefense:
		return domain.EventKindFinishDefense, true
	case eventNameFinishTake:
		return domain.EventKindFinishTake, true
	case eventNameRefill:
		return domain.EventKindRefill, true
	case eventNameRoundEnded:
		return domain.EventKindRoundEnded, true
	case eventNameConcede:
		return domain.EventKindConcede, true
	case eventNameMatchEnded:
		return domain.EventKindMatchEnded, true
	default:
		return 0, false
	}
}

func encodeActionKind(kind domain.ActionKind) (string, error) {
	switch kind {
	case domain.ActionKindAttack:
		return eventNameAttack, nil
	case domain.ActionKindDefend:
		return eventNameDefend, nil
	case domain.ActionKindThrowIn:
		return eventNameThrowIn, nil
	case domain.ActionKindTake:
		return eventNameTake, nil
	case domain.ActionKindFinishDefense:
		return eventNameFinishDefense, nil
	case domain.ActionKindFinishTake:
		return eventNameFinishTake, nil
	case domain.ActionKindTransfer:
		return eventNameTransfer, nil
	default:
		return "", fmt.Errorf("%w: unknown action kind %d", ErrInvalidEventEnvelope, kind)
	}
}

func decodeActionKind(name string) (domain.ActionKind, error) {
	switch name {
	case eventNameAttack:
		return domain.ActionKindAttack, nil
	case eventNameDefend:
		return domain.ActionKindDefend, nil
	case eventNameThrowIn:
		return domain.ActionKindThrowIn, nil
	case eventNameTake:
		return domain.ActionKindTake, nil
	case eventNameFinishDefense:
		return domain.ActionKindFinishDefense, nil
	case eventNameFinishTake:
		return domain.ActionKindFinishTake, nil
	case eventNameTransfer:
		return domain.ActionKindTransfer, nil
	default:
		return 0, fmt.Errorf("%w: unknown action kind %q", ErrInvalidEventEnvelope, name)
	}
}

func actionHasCard(kind domain.ActionKind) bool {
	switch kind {
	case domain.ActionKindAttack, domain.ActionKindDefend, domain.ActionKindThrowIn, domain.ActionKindTransfer:
		return true
	default:
		return false
	}
}

func encodeRoundOutcome(outcome domain.RoundOutcome) (string, error) {
	switch outcome {
	case domain.RoundOutcomeDefense:
		return roundOutcomeDefense, nil
	case domain.RoundOutcomeTake:
		return roundOutcomeTake, nil
	default:
		return "", fmt.Errorf("%w: unknown round outcome %d", ErrInvalidEventEnvelope, outcome)
	}
}

func decodeRoundOutcome(outcome string) (domain.RoundOutcome, error) {
	switch outcome {
	case roundOutcomeDefense:
		return domain.RoundOutcomeDefense, nil
	case roundOutcomeTake:
		return domain.RoundOutcomeTake, nil
	default:
		return domain.RoundOutcomeUnknown, fmt.Errorf("%w: unknown round outcome %q", ErrInvalidEventEnvelope, outcome)
	}
}
