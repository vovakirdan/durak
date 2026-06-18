# Durak Evaluation Model

Status: current implemented model.

The evaluator is a fair seat-view risk model. It does not use hidden opponent
hands or stock order. It reads the evaluated seat's hand, public table state,
visible counters, legal actions, and `app.PublicCardMemory`.

The old `base position + action delta` model has been removed from the
production path. The current model treats the previous evaluator as absent and
keeps only useful public contracts:

- `Score`
- `MoveQuality`
- `PositionEvaluation`
- `ActionEvaluation`
- `Evaluate`
- `RankActions`

## Score Scale

Scores are clamped to `[-1000, +1000]`.

The model estimates the evaluated seat's probability of becoming durak. With
`N` active players, neutral durak probability is `1/N`.

```text
P(durak) = 0      -> +1000
P(durak) = 1 / N  -> 0
P(durak) = 1      -> -1000
```

Implementation: `ScoreFromDurakProbability`.

## Risk Model

Each active seat receives a risk index measured in approximate bad-card units.

```text
Risk =
    hand_burden
  + battle_threat
  - outlet
  - defense_stability
  - initiative
```

The risk vector is converted to durak probabilities with softmax:

```text
P_i = exp(beta * Risk_i) / sum(exp(beta * Risk_j))
```

The evaluated seat's probability is then mapped to the public score scale.

Implemented feature names:

- `risk_score`
- `risk_hand_burden`
- `risk_battle_threat`
- `risk_outlet`
- `risk_defense_stability`
- `risk_initiative`

Weights live in `internal/app/evaluation/risk.go` so the Excel model can be
ported by changing one small struct.

## Information Model

`BuildHiddenCards` builds a fair hidden-card view from:

- public memory when available;
- own hand;
- table cards;
- trump indicator;
- discard cards passed by analysis;
- cards known to be held because a player visibly took them.

Unknown cards are `deck - known`.

This is intentionally compatible with a future god-mode evaluator: god mode
should supply a sharper hidden-card model, not a different evaluator API.

## Cover Probability

For one pending attack, the evaluator uses the closed-form probability that the
defender has at least one beating card in a hidden hand sampled without
replacement from the unknown pool.

For multiple pending attacks, the evaluator checks whether distinct defense
cards can cover every attack. The matching check is a small DFS over the current
hand shape, with deterministic sampling for unknown cards. No dependency is
used.

Implementation:

- `CoverProbability`
- `CanCoverAll`

## Battle Risk

The local defender problem compares:

```text
take now
continue defense
transfer
```

`EvaluateBattleRisk` returns all three branch costs plus the best branch and
cover probability.

Defense card cost includes:

- card spend cost;
- extra cost for trump reserve;
- rank-opening pressure from unknown and known-held cards of the defense
  card's rank.

Taking cost includes:

- cards currently on the table as incoming burden;
- a skip/initiative penalty.

Successful defense receives a small initiative credit.

## Action Ranking

`RankActions` no longer ranks actions by adding a hand-written delta to the
current static score.

For each legal action:

1. `ScoreAction` projects the minimal public effect of the action.
2. Attack and throw-in actions are evaluated as a mixture of:
   - defender covers all pending attacks;
   - defender takes the table.
3. Deterministic actions such as defend, transfer, take, finish defense, finish
   take, and pass are scored from their projected public state.
4. A small resource penalty keeps irreversible card spending visible to the
   ranker.
5. Scores are sorted descending and converted to `Loss` and `MoveQuality`.

The projection is deliberately shallow. It is a battle-local comparison, not
MCTS and not full game search.

## Arena Diagnostics

`arena -eval` still prints aggregate move-quality counters.

`arena -eval-log <path>` writes JSONL rows with:

- chosen action and top ranked action;
- chosen action rank and loss;
- current risk score;
- battle take/defend/transfer branch costs;
- cover probability when applicable.

The log is diagnostic data for calibration, not a public gameplay event stream.

## Known Gaps

- The first risk-model pass is runtime-stable but not yet calibrated to beat
  the deterministic `simple` baseline in mirrored arena runs.
- Initial attack is still one `domain.Action` per card. The research model's
  `AttackPacket(rank, cards)` is not implemented because the domain legal-action
  API does not expose packet attacks yet.
- Refill projection approximates attack participants from visible roles. The
  current `DecisionContext` does not expose full attack participant order.
- Multi-seat throw-in pressure is approximate.
- Cover probability uses deterministic sampling for larger hidden pools instead
  of exhaustive belief enumeration.
- No god-mode evaluator is implemented.
- No learned weights are implemented.
