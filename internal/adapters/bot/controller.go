package bot

import (
	"errors"
	"fmt"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

const (
	// ControllerSimple is the deterministic baseline controller.
	ControllerSimple = "simple"
	// ControllerRandom chooses uniformly from legal actions.
	ControllerRandom = "random"
	// ControllerHeuristic chooses the highest ranked action from position evaluation.
	ControllerHeuristic = "heuristic"
)

var (
	// ErrUnknownController means a bot controller kind is not registered.
	ErrUnknownController = errors.New("unknown bot controller")
	// ErrUnsupportedControllerSeat means deterministic controller seeding cannot cover the seat.
	ErrUnsupportedControllerSeat = errors.New("unsupported controller seat")
)

// ControllerSpec selects one bot controller implementation.
type ControllerSpec struct {
	Kind   string
	Seed   uint64
	Seeded bool
}

// DefaultControllerSpec returns the MVP bot controller configuration.
func DefaultControllerSpec() ControllerSpec {
	return ControllerSpec{Kind: ControllerSimple}
}

// NewController creates a player controller from an adapter-level spec.
func NewController(spec ControllerSpec, seat domain.Seat) (app.PlayerController, error) {
	kind := normalizeControllerKind(spec.Kind)
	switch kind {
	case ControllerSimple:
		return app.StrategyController{Strategy: NewSimpleStrategy()}, nil
	case ControllerHeuristic:
		return NewHeuristicController(), nil
	case ControllerRandom:
		if !spec.Seeded {
			return NewRandomLegalController(nil), nil
		}
		seed, err := controllerSeatSeed(spec.Seed, seat)
		if err != nil {
			return nil, err
		}
		random := domain.NewSeededRandom(seed)
		return NewRandomLegalController(random.Intn), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownController, kind)
	}
}

// ValidateControllerKind rejects unknown bot controller names.
func ValidateControllerKind(kind string) error {
	kind = normalizeControllerKind(kind)
	switch kind {
	case ControllerSimple, ControllerRandom, ControllerHeuristic:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnknownController, kind)
	}
}

func normalizeControllerKind(kind string) string {
	if kind == "" {
		return ControllerSimple
	}
	return kind
}

func controllerSeatSeed(seed uint64, seat domain.Seat) (uint64, error) {
	switch seat {
	case domain.Seat(0):
		return seed + 1, nil
	case domain.Seat(1):
		return seed + 2, nil
	case domain.Seat(2):
		return seed + 3, nil
	case domain.Seat(3):
		return seed + 4, nil
	case domain.Seat(4):
		return seed + 5, nil
	case domain.Seat(5):
		return seed + 6, nil
	default:
		return 0, fmt.Errorf("%w: %d", ErrUnsupportedControllerSeat, seat)
	}
}
