package domain

// CanBeat reports whether defense can beat attack with the given trump suit.
func CanBeat(attack, defense Card, trumpSuit Suit) bool {
	if attack.Suit == defense.Suit {
		return defense.Rank > attack.Rank
	}
	return defense.Suit == trumpSuit && attack.Suit != trumpSuit
}
