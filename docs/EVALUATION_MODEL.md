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
    W_hand * hand_burden
  + W_battle * battle_risk
  - W_outlet * outlet
  - W_defense * defense_stability
  - W_initiative * initiative
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

Default weights are aligned with the spreadsheet model:

```text
beta = 0.30
W_hand = 1.00
W_battle = 1.20
W_outlet = 0.80
W_defense = 0.90
W_initiative = 0.50
```

Card-cost constants are also spreadsheet-aligned: trump premium `2.5`,
rank-opening base `0.5`, skip penalty `2.0`, and initiative bonus `1.5`.

`hand_burden` uses the workbook phase weight:

```text
e = cards_in_hands / (cards_in_hands + stock_count)
phase_weight = 0.35 + 0.65 * e
hand_burden = phase_weight * sum(card_stickiness)
```

As the stock disappears, exact card stickiness matters more.

## Information Model

`BuildHiddenCards` builds a fair hidden-card view from:

- public memory when available;
- own hand;
- table cards;
- trump indicator;
- public discard memory;
- discard cards passed by analysis;
- cards known to be held because a player visibly took them.

Action projection preserves this memory. If a projected battle ends with a
take, the visible table cards are added to the defender's known held cards
before the boundary position is scored.

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

- `(rank_value + 1) / 9`;
- trump reserve loss;
- rank-opening pressure from the workbook free-rank count.

Taking cost includes:

- cards currently on the table as incoming burden;
- a skip/initiative penalty.

The selected branch is exposed as `BestBranch` and is used by the battle
resolver.

## Position Terms

`battle_risk` follows the workbook rows:

```text
current defender: min(take_now, continue_defense, transfer) + 0.3 * table_pressure
passive active seat: 0.5
attacker: 0
```

`outlet` follows the workbook total: low non-trump attack cards plus `0.5` per
rank group with at least two cards. Throw-in cards are scored through action
projection, not as a static position term.

`defense_stability` is the workbook stability formula:

```text
(trump_count * 1.5 + high_non_trump_count * 0.5) / max(1, hand_size)
```

`initiative` is `+1.5` for the current attacker, `-0.5` for the current
defender, and `0` for every other active seat.

## Attack Packets

Initial attack can be a packet of same-rank cards. `domain.Action` keeps the
legacy `Card` field for one-card actions and stores packet attacks in a fixed
array so actions remain comparable and existing validation paths still work.

Legal attack actions are generated in this order:

1. one-card attacks in hand order;
2. same-rank packet attacks that fit the current attack limit.

Text commands and traces use:

```text
attack 6C 6D
```

Public event JSON remains backward-compatible: old one-card attacks use
`card`, packet attacks use `cards`.

## Action Ranking

`RankActions` no longer ranks actions by adding a hand-written delta to the
current static score.

For each legal action:

1. `ScoreAction` applies the candidate action to a projected public state.
2. `ResolveBattleExpected` resolves the current battle to a common round
   boundary by selecting the defender's minimum-risk branch:
   - defend all pending attacks;
   - take the table;
   - transfer to the next defender, with a bounded transfer chain.
3. The resolved boundary position is scored with the risk model.
4. A small local release-cost term keeps irreversible card spending visible to
   the ranker, using the same spend-cost units as the battle model.
5. Known taken cards stay visible in the projected position, so feeding a
   defender useful cards can reduce the attacker's score.
6. `HelpfulPickupRisk` adds a small explicit penalty when attack or throw-in is
   expected to end with the defender taking useful cards and the action does
   not immediately win.
7. Scores are sorted descending and converted to `Loss` and `MoveQuality`.

The projection is deliberately shallow. It is a battle-local comparison, not
MCTS and not full game search.

## Arena Diagnostics

`arena -eval` still prints aggregate move-quality counters.

`arena -eval-log <path>` writes JSONL rows with:

- chosen action and top ranked action;
- chosen score and top score;
- chosen action rank and loss;
- current risk score;
- expected battle response after the chosen action;
- battle take/defend/transfer branch costs;
- cover probability when applicable.
- current risk components.

The log is diagnostic data for calibration, not a public gameplay event stream.

## Known Gaps

- Current weights and action-local penalties are not calibrated to beat the
  deterministic `simple` baseline in mirrored arena runs yet.
- Refill projection approximates attack participants from visible roles. The
  current `DecisionContext` does not expose full attack participant order.
- Multi-seat throw-in pressure is approximate.
- Cover probability uses deterministic sampling for larger hidden pools instead
  of exhaustive belief enumeration.
- The spreadsheet parity test locks the battle-cost fixture exactly and score
  direction broadly; hand-burden cached values in the workbook are not used as a
  runtime dependency.
- No god-mode evaluator is implemented.
- No learned weights are implemented.
