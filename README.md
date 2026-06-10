# durak

Terminal-first Durak game, starting with a local CLI MVP and designed to grow
into a Bubble Tea TUI and Wish-hosted SSH multiplayer daemon.

## Status

Bootstrap skeleton only. The game loop and rules engine are not implemented yet.

## Development

```sh
make check
make run
make build
```

The Makefile keeps Go build caches under `.cache/` so commands work in
restricted workspaces without writing to the user-level Go cache.

## Planning Docs

- [PRD](docs/2026-06-10-durak-prd.md)
- [Stack](docs/STACK.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Project Rules](docs/PROJECT_RULES.md)
