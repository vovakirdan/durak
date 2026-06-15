# Match Configuration Draft

Status: Draft.

## Purpose

The project needs a per-match configuration model before it needs an external
file format. The first implementation is an in-process Go value object that can
be constructed from built-in presets, CLI flags, tests, future storage records,
or a daemon control plane. YAML/JSON syntax is intentionally deferred because a
future daemon may load match configs from SQLite or another live source without
restarting a runner.

The config model is not the rule engine. It validates and maps into
`domain.RuleProfile`; the domain still owns legal moves and state transitions.
Unsupported future variants should be representable in the model where useful,
but must fail clearly before a match starts.

## Current Model

- `MatchConfig` owns the app-level match creation contract.
- `RuleConfig` owns game-rule configuration and maps to `domain.RuleProfile`.
- `SeatConfig` owns table occupancy for a single match.
- `SeriesConfig` owns consecutive-match behavior that needs previous result
  state.
- Bot difficulty, AI provider settings, event-log paths, seeds, and controller
  selection stay outside rules config.

The initial built-in preset is `default`. It is a Go preset, not an external
file. It preserves the current house rules while making the fields explicit.

## Configurable Areas

Table and match:

- player count and maximum seats;
- stable seat order, currently canonical `0..n-1`;
- turn direction, currently clockwise only;
- independent match versus consecutive series;
- consecutive first-attacker override: seat before previous loser.

Deck and setup:

- deck layout, currently 36 cards;
- initial hand size;
- same-suit redeal threshold;
- trump indicator selection policy;
- forbidden trump indicator rank;
- forbidden trump behavior, currently stock-only reshuffle/reselect;
- maximum setup attempts.

First attacker:

- lowest trump policy;
- random fallback when no player has trump;
- series override from previous loser metadata.

Attack and throw-ins:

- whether throw-ins are enabled;
- eligible throw-in seats: lead only, neighbors, or all except defender;
- throw-in timing: free-form any eligible now, ordered timing later;
- opening rule: lead attacker first or any eligible;
- close rule: lead may close or all eligible seats must pass;
- legal rank rule: rank must already be on the table;
- first successful defense attack limit;
- attack limit policy, including defender-initial-hand cap;
- whether additional throw-ins are allowed after defender chooses to take.

Defense, transfer, refill, and completion:

- whether taking is available;
- whether taking after partial defense is available;
- whether transfer is enabled;
- whether the first attack can be transferred;
- whether transfer after defense is allowed, currently unsupported;
- transfer rank and target policies;
- refill order, currently attack participants then defender;
- draw outcome handling when all remaining players run out together.

Future options:

- move and throw-in time limits;
- timeout action policy;
- cheating rules;
- other deck layouts or custom rank ranges.

## Runtime Loading Direction

Future config sources should provide immutable config values to match creation.
The active match should keep the rule profile it was created with. Updating a
preset or DB row should affect new matches only unless a deliberate migration or
admin action says otherwise. This keeps replay stable: a match event stream must
record enough config identity/version data to reconstruct the rules that were
active at creation time.
