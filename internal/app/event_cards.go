package app

import (
	"fmt"

	"github.com/vovakirdan/durak/internal/domain"
)

func encodeCard(card domain.Card) (cardPayload, error) {
	rank, err := encodeRank(card.Rank)
	if err != nil {
		return cardPayload{}, err
	}
	suit, err := encodeSuit(card.Suit)
	if err != nil {
		return cardPayload{}, err
	}
	return cardPayload{Rank: rank, Suit: suit}, nil
}

func decodeCard(card cardPayload) (domain.Card, error) {
	rank, err := decodeRank(card.Rank)
	if err != nil {
		return domain.Card{}, err
	}
	suit, err := decodeSuit(card.Suit)
	if err != nil {
		return domain.Card{}, err
	}
	return domain.Card{Rank: rank, Suit: suit}, nil
}

func encodeCards(cards []domain.Card) ([]cardPayload, error) {
	encoded := make([]cardPayload, len(cards))
	for i, card := range cards {
		encodedCard, err := encodeCard(card)
		if err != nil {
			return nil, err
		}
		encoded[i] = encodedCard
	}
	return encoded, nil
}

func decodeCards(cards []cardPayload) ([]domain.Card, error) {
	decoded := make([]domain.Card, len(cards))
	for i, card := range cards {
		decodedCard, err := decodeCard(card)
		if err != nil {
			return nil, err
		}
		decoded[i] = decodedCard
	}
	return decoded, nil
}

func encodeSuit(suit domain.Suit) (string, error) {
	switch suit {
	case domain.Clubs:
		return "C", nil
	case domain.Diamonds:
		return "D", nil
	case domain.Hearts:
		return "H", nil
	case domain.Spades:
		return "S", nil
	default:
		return "", fmt.Errorf("%w: unknown suit %d", ErrInvalidEventEnvelope, suit)
	}
}

func decodeSuit(suit string) (domain.Suit, error) {
	switch suit {
	case "C":
		return domain.Clubs, nil
	case "D":
		return domain.Diamonds, nil
	case "H":
		return domain.Hearts, nil
	case "S":
		return domain.Spades, nil
	default:
		return domain.SuitUnknown, fmt.Errorf("%w: unknown suit %q", ErrInvalidEventEnvelope, suit)
	}
}

func encodeRank(rank domain.Rank) (string, error) {
	switch rank {
	case domain.Six:
		return "6", nil
	case domain.Seven:
		return "7", nil
	case domain.Eight:
		return "8", nil
	case domain.Nine:
		return "9", nil
	case domain.Ten:
		return "10", nil
	case domain.Jack:
		return "J", nil
	case domain.Queen:
		return "Q", nil
	case domain.King:
		return "K", nil
	case domain.Ace:
		return "A", nil
	default:
		return "", fmt.Errorf("%w: unknown rank %d", ErrInvalidEventEnvelope, rank)
	}
}

func decodeRank(rank string) (domain.Rank, error) {
	switch rank {
	case "6":
		return domain.Six, nil
	case "7":
		return domain.Seven, nil
	case "8":
		return domain.Eight, nil
	case "9":
		return domain.Nine, nil
	case "10":
		return domain.Ten, nil
	case "J":
		return domain.Jack, nil
	case "Q":
		return domain.Queen, nil
	case "K":
		return domain.King, nil
	case "A":
		return domain.Ace, nil
	default:
		return domain.RankUnknown, fmt.Errorf("%w: unknown rank %q", ErrInvalidEventEnvelope, rank)
	}
}
