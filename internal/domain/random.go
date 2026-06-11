package domain

import rand "math/rand/v2"

const seededRandomSalt = 0x9e3779b97f4a7c15

// SeededRandom provides repeatable setup randomness for deals and tests.
type SeededRandom struct {
	rng *rand.Rand
}

// NewSeededRandom returns repeatable randomness derived from seed.
func NewSeededRandom(seed uint64) *SeededRandom {
	return &SeededRandom{
		// #nosec G404 -- replayable game setup randomness is not used for security.
		rng: rand.New(rand.NewPCG(seed, seed^seededRandomSalt)),
	}
}

// SeededDealOptions returns deal options that replay the same setup for seed.
func SeededDealOptions(seed uint64) DealOptions {
	random := NewSeededRandom(seed)
	return DealOptions{
		Shuffler: random,
		Choose:   random.Intn,
	}
}

// Shuffle mutates cards into a repeatable pseudo-random order.
func (r *SeededRandom) Shuffle(cards []Card) {
	r.rng.Shuffle(len(cards), func(i, j int) {
		cards[i], cards[j] = cards[j], cards[i]
	})
}

// Intn returns a repeatable integer in [0, n).
func (r *SeededRandom) Intn(n int) int {
	return r.rng.IntN(n)
}
