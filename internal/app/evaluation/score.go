package evaluation

// Score is a chess-like positional score from one seat's point of view.
type Score int

const (
	// MinScore is the lower practical bound for a decisive disadvantage.
	MinScore Score = -1000
	// MaxScore is the upper practical bound for a decisive advantage.
	MaxScore Score = 1000
)

// Clamp keeps a score inside the public evaluation scale.
func Clamp(score Score) Score {
	if score < MinScore {
		return MinScore
	}
	if score > MaxScore {
		return MaxScore
	}
	return score
}

// MoveQuality labels an action by loss versus the best ranked action.
type MoveQuality string

const (
	// MoveQualityBest means the action is effectively tied with the top action.
	MoveQualityBest MoveQuality = "best"
	// MoveQualityGood means the action loses only a small amount of score.
	MoveQualityGood MoveQuality = "good"
	// MoveQualityInaccuracy means the action gives up a visible positional edge.
	MoveQualityInaccuracy MoveQuality = "inaccuracy"
	// MoveQualityMistake means the action is a large avoidable score loss.
	MoveQualityMistake MoveQuality = "mistake"
	// MoveQualityBlunder means the action loses a decisive amount of score.
	MoveQualityBlunder MoveQuality = "blunder"
	// MoveQualityBrilliant is reserved for future search-backed non-obvious moves.
	MoveQualityBrilliant MoveQuality = "brilliant"
)

// QualityFromLoss maps score loss to a stable training label.
func QualityFromLoss(loss Score) MoveQuality {
	switch {
	case loss <= 20:
		return MoveQualityBest
	case loss <= 80:
		return MoveQualityGood
	case loss <= 180:
		return MoveQualityInaccuracy
	case loss <= 350:
		return MoveQualityMistake
	default:
		return MoveQualityBlunder
	}
}
