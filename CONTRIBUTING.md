# Contributing

## Setup

- Install Go 1.24 or newer.
- Install Node 24.13.0 and npm 11.12.0 or newer if you are changing the Raycast workspace.
- Install a local Codex CLI/Desktop environment if you want to test against real session data.
- Clone the repo and run:

```bash
make tidy
make test
```

If you are working on the Raycast extension, also run:

```bash
cd raycast
npm install
npm test
```

## Development Workflow

- Use `make build` to produce `./bin/codex-chat-manager`.
- Use `make tui` or `make list` to exercise the app against your local `CODEX_HOME`.
- Use `cd raycast && npm run lint` and `cd raycast && npm run build` for Raycast extension changes.
- Prefer copied or synthetic fixtures in tests instead of mutating your real `~/.codex`.

## Verification

Run the full non-mutating verification pass before opening a PR:

```bash
make verify
```

If you intentionally changed formatting or dependencies, run:

```bash
make fmt
make tidy
make verify
```

For Raycast changes, also run:

```bash
cd raycast
npm run lint
npm run build
```

## Scope

- Keep rollout JSONL files as the source of truth for inventory and destructive actions.
- Do not add SQLite mutation unless it is explicitly part of the change.
- Keep archive, unarchive, delete, and grouping semantics aligned across both the CLI and the TUI.
- Keep the Raycast extension layered on top of existing CLI JSON outputs instead of reimplementing list or mutation semantics independently.
