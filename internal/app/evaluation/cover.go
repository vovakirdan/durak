package evaluation

import (
	"hash/fnv"
	"math/rand/v2"
	"slices"
	"strconv"

	"github.com/vovakirdan/durak/internal/domain"
)

const coverSamples = 128

// CoverProbability estimates whether defender can cover all pending attacks
// from known-held cards plus a fair unknown-card hand.
func CoverProbability(
	pending []domain.Card,
	defender domain.Seat,
	defenderHandSize int,
	trump domain.Suit,
	hidden HiddenCards,
) float64 {
	pending = validCardsOnly(pending)
	if len(pending) == 0 {
		return 1
	}
	known := knownHeldForSeat(hidden, defender)
	if CanCoverAll(pending, known, trump) {
		return 1
	}
	hiddenCount := defenderHandSize - len(known)
	if hiddenCount <= 0 || len(hidden.UnknownPool) == 0 {
		return 0
	}
	if hiddenCount > len(hidden.UnknownPool) {
		hiddenCount = len(hidden.UnknownPool)
	}
	if len(pending) == 1 {
		return singleCoverProbability(pending[0], known, hidden.UnknownPool, hiddenCount, trump)
	}
	return sampledCoverProbability(pending, known, hidden.UnknownPool, hiddenCount, trump)
}

// CanCoverAll reports whether distinct defense cards can beat every attack.
func CanCoverAll(pending, hand []domain.Card, trump domain.Suit) bool {
	pending = validCardsOnly(pending)
	hand = validCardsOnly(hand)
	if len(pending) == 0 {
		return true
	}
	if len(hand) < len(pending) {
		return false
	}
	used := make([]bool, len(hand))
	var search func(int) bool
	search = func(index int) bool {
		if index == len(pending) {
			return true
		}
		for handIndex, card := range hand {
			if used[handIndex] || !domain.CanBeat(pending[index], card, trump) {
				continue
			}
			used[handIndex] = true
			if search(index + 1) {
				return true
			}
			used[handIndex] = false
		}
		return false
	}
	return search(0)
}

func singleCoverProbability(
	attack domain.Card,
	known []domain.Card,
	unknown []domain.Card,
	hiddenCount int,
	trump domain.Suit,
) float64 {
	for _, card := range known {
		if domain.CanBeat(attack, card, trump) {
			return 1
		}
	}
	beaters := 0
	for _, card := range unknown {
		if domain.CanBeat(attack, card, trump) {
			beaters++
		}
	}
	return hypergeometricAtLeastOne(len(unknown), beaters, hiddenCount)
}

func hypergeometricAtLeastOne(pool, hits, draws int) float64 {
	if pool <= 0 || hits <= 0 || draws <= 0 {
		return 0
	}
	if draws > pool {
		draws = pool
	}
	misses := pool - hits
	if draws > misses {
		return 1
	}
	noHit := 1.0
	for i := range draws {
		noHit *= float64(misses-i) / float64(pool-i)
	}
	return 1 - noHit
}

func sampledCoverProbability(
	pending []domain.Card,
	known []domain.Card,
	unknown []domain.Card,
	hiddenCount int,
	trump domain.Suit,
) float64 {
	if hiddenCount <= 0 {
		return 0
	}
	seed := coverSeed(pending, known, unknown, hiddenCount, trump)
	// #nosec G404 -- deterministic simulation, not security-sensitive randomness.
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	covered := 0
	for range coverSamples {
		sample := sampleCards(unknown, hiddenCount, rng)
		hand := append(slices.Clone(known), sample...)
		if CanCoverAll(pending, hand, trump) {
			covered++
		}
	}
	return float64(covered) / coverSamples
}

func sampleCards(cards []domain.Card, count int, rng *rand.Rand) []domain.Card {
	if count <= 0 || len(cards) == 0 {
		return nil
	}
	if count > len(cards) {
		count = len(cards)
	}
	pool := slices.Clone(cards)
	for i := range count {
		j := i + rng.IntN(len(pool)-i)
		pool[i], pool[j] = pool[j], pool[i]
	}
	return pool[:count]
}

func coverSeed(
	pending []domain.Card,
	known []domain.Card,
	unknown []domain.Card,
	hiddenCount int,
	trump domain.Suit,
) uint64 {
	h := fnv.New64a()
	hashInt(h, int(trump))
	hashInt(h, hiddenCount)
	for _, card := range sortedCardsForCover(pending) {
		hashCard(h, card)
	}
	for _, card := range sortedCardsForCover(known) {
		hashCard(h, card)
	}
	for _, card := range sortedCardsForCover(unknown) {
		hashCard(h, card)
	}
	return h.Sum64()
}

func knownHeldForSeat(hidden HiddenCards, seat domain.Seat) []domain.Card {
	groups := hidden.knownHeldGroups()
	if seat == domain.NoSeat || int(seat) < 0 || int(seat) >= len(groups) {
		return nil
	}
	return groups[int(seat)]
}

func validCardsOnly(cards []domain.Card) []domain.Card {
	valid := make([]domain.Card, 0, len(cards))
	for _, card := range cards {
		if validCard(card) {
			valid = append(valid, card)
		}
	}
	return valid
}

func sortedCardsForCover(cards []domain.Card) []domain.Card {
	sorted := slices.Clone(cards)
	slices.SortFunc(sorted, func(left, right domain.Card) int {
		if left.Suit != right.Suit {
			return int(left.Suit - right.Suit)
		}
		return int(left.Rank - right.Rank)
	})
	return sorted
}

func hashCard(h interface{ Write([]byte) (int, error) }, card domain.Card) {
	hashInt(h, int(card.Rank))
	hashInt(h, int(card.Suit))
}

func hashInt(h interface{ Write([]byte) (int, error) }, value int) {
	data := strconv.AppendInt(nil, int64(value), 10)
	data = append(data, ',')
	if _, err := h.Write(data); err != nil {
		panic(err)
	}
}
