package app

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/vovakirdan/durak/internal/domain"
)

// ErrInvalidReplay means an internal event stream cannot be replayed exactly.
var ErrInvalidReplay = errors.New("invalid replay")

// ReplayResult contains the reconstructed match and the domain events it emitted.
type ReplayResult struct {
	Match  *domain.Match
	Events []domain.Event
}

// ReplayInternalEvents reconstructs a match from its canonical internal stream.
func ReplayInternalEvents(events []InternalEvent, profile domain.RuleProfile) (ReplayResult, error) {
	if len(events) == 0 {
		return ReplayResult{}, fmt.Errorf("%w: event stream is empty", ErrInvalidReplay)
	}
	if err := validateReplaySequence(events); err != nil {
		return ReplayResult{}, err
	}

	var match *domain.Match
	replayedEvents := make([]domain.Event, 0, len(events))
	for i := range events {
		event := events[i]
		switch event.Domain.Kind {
		case domain.EventKindMatchStarted:
			continue
		case domain.EventKindDeal:
			if match != nil {
				return ReplayResult{}, fmt.Errorf("%w: duplicate deal event at sequence %d", ErrInvalidReplay, event.Sequence)
			}
			if event.Deal == nil {
				return ReplayResult{}, fmt.Errorf("%w: missing internal deal at sequence %d", ErrInvalidReplay, event.Sequence)
			}
			var err error
			deal := event.Deal.InitialDeal()
			match, err = domain.NewMatch(&deal, profile)
			if err != nil {
				return ReplayResult{}, fmt.Errorf("%w: create match at sequence %d: %w", ErrInvalidReplay, event.Sequence, err)
			}
			replayedEvents = append(replayedEvents, match.DrainEvents()...)
		case domain.EventKindAttack, domain.EventKindDefend, domain.EventKindThrowIn,
			domain.EventKindTransfer, domain.EventKindTake, domain.EventKindFinishDefense,
			domain.EventKindFinishTake:
			if match == nil {
				return ReplayResult{}, fmt.Errorf("%w: action before deal at sequence %d", ErrInvalidReplay, event.Sequence)
			}
			if event.Domain.Action == nil {
				return ReplayResult{}, fmt.Errorf("%w: missing action payload at sequence %d", ErrInvalidReplay, event.Sequence)
			}
			if err := match.ApplyAction(event.Domain.Action.Action); err != nil {
				return ReplayResult{}, fmt.Errorf("%w: apply action at sequence %d: %w", ErrInvalidReplay, event.Sequence, err)
			}
			replayedEvents = append(replayedEvents, match.DrainEvents()...)
		case domain.EventKindConcede:
			if match == nil {
				return ReplayResult{}, fmt.Errorf("%w: concede before deal at sequence %d", ErrInvalidReplay, event.Sequence)
			}
			if event.Domain.Concede == nil {
				return ReplayResult{}, fmt.Errorf("%w: missing concede payload at sequence %d", ErrInvalidReplay, event.Sequence)
			}
			if err := match.Concede(event.Domain.Concede.Seat); err != nil {
				return ReplayResult{}, fmt.Errorf("%w: concede at sequence %d: %w", ErrInvalidReplay, event.Sequence, err)
			}
			replayedEvents = append(replayedEvents, match.DrainEvents()...)
		case domain.EventKindRefill, domain.EventKindRoundEnded, domain.EventKindMatchEnded:
			continue
		default:
			return ReplayResult{}, fmt.Errorf("%w: unknown event kind %d at sequence %d", ErrInvalidReplay, event.Domain.Kind, event.Sequence)
		}
	}
	if match == nil {
		return ReplayResult{}, fmt.Errorf("%w: missing deal event", ErrInvalidReplay)
	}
	if !reflect.DeepEqual(replayedEvents, replaySourceEvents(events)) {
		return ReplayResult{}, fmt.Errorf("%w: replayed events differ from source", ErrInvalidReplay)
	}

	return ReplayResult{Match: match, Events: replayedEvents}, nil
}

func validateReplaySequence(events []InternalEvent) error {
	matchID := events[0].MatchID
	if matchID == "" {
		return fmt.Errorf("%w: match id is empty", ErrInvalidReplay)
	}
	for i := range events {
		event := events[i]
		if event.MatchID != matchID {
			return fmt.Errorf("%w: event %d match id = %q, want %q", ErrInvalidReplay, i, event.MatchID, matchID)
		}
		sequence := uint64(i + 1)
		if event.Sequence != sequence {
			return fmt.Errorf("%w: event %d sequence = %d, want %d", ErrInvalidReplay, i, event.Sequence, sequence)
		}
	}
	return nil
}

func replaySourceEvents(events []InternalEvent) []domain.Event {
	source := make([]domain.Event, len(events))
	for i := range events {
		source[i] = events[i].Domain.Clone()
	}
	return source
}
