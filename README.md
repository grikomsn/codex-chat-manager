# codex-chat-manager

CLI and TUI for managing local OpenAI Codex chat sessions.

The app reads Codex rollout files directly from disk and treats them as the source of truth. It supports active and archived sessions, parent/subagent grouping, transcript preview, archive/unarchive, resume, and bulk delete.

## Why

Codex stores valuable local session history, but the built-in storage is file-oriented and not optimized for bulk management. This project provides a focused local manager for browsing, previewing, grouping subagent threads, and performing safe archive or delete workflows on top of the rollout files Codex already writes.

## Prerequisites

- Go 1.24 or newer if building from source
- A local Codex installation that writes session files under `CODEX_HOME`
- A session tree that matches current Codex rollout conventions such as `sessions/YYYY/MM/DD/rollout-...jsonl` and `archived_sessions/rollout-...jsonl`

## Storage Model

By default the app uses `~/.codex`.

It reads:

- `~/.codex/sessions/YYYY/MM/DD/rollout-...jsonl`
- `~/.codex/archived_sessions/rollout-...jsonl`
- `~/.codex/session_index.jsonl` as a best-effort title cache only

It does not use SQLite as the source of truth and does not mutate SQLite on delete.

The Codex on-disk format is reverse-engineered from upstream behavior and may drift over time. The tool intentionally treats rollout files as canonical, `session_index.jsonl` as advisory only, and SQLite as out of scope.

## Build

```bash
go build ./cmd/codex-chat-manager
```

Install directly:

```bash
go install github.com/grikomsn/codex-chat-manager/cmd/codex-chat-manager@latest
```

## Run

Start the TUI:

```bash
go run ./cmd/codex-chat-manager
```

Override the Codex home:

```bash
go run ./cmd/codex-chat-manager tui --codex-home /path/to/codex-home
```

You can also set `CODEX_HOME`.

The app can operate on a copied Codex home if you want to inspect or test against a safe fixture instead of your live `~/.codex`.

## CLI

List sessions:

```bash
go run ./cmd/codex-chat-manager sessions list
go run ./cmd/codex-chat-manager sessions list --status archived --filter bundle
go run ./cmd/codex-chat-manager sessions list --json --include-children
```

Archive or unarchive:

```bash
go run ./cmd/codex-chat-manager sessions archive --id SESSION_ID
go run ./cmd/codex-chat-manager sessions unarchive --id SESSION_ID
```

Delete archived sessions:

```bash
go run ./cmd/codex-chat-manager sessions delete --id SESSION_ID --yes
```

Resume an active session:

```bash
go run ./cmd/codex-chat-manager sessions resume --id SESSION_ID
```

## TUI

Wide terminals show a session list on the left and preview on the right.

Narrow terminals show the list first; press `enter` to open the preview and `esc` to return.

Key bindings:

- `j` / `k` or arrows: move
- `space`: toggle selection
- `tab`: cycle `all`, `active`, `archived`
- `enter`, `l`, right arrow: open group drill-in or preview
- `h`, left arrow, `esc`: back
- `a`: archive selected sessions
- `u`: unarchive selected sessions
- `d`: delete selected archived sessions
- `r`: resume the highlighted active session
- `ctrl+r`: refresh from disk
- `?`: toggle help
- `q`: quit

## Safety Rules

- Sessions are discovered from rollout files, not from `session_index.jsonl`.
- Parent session actions cascade to grouped subagent children.
- Deleting active sessions is blocked. Archive first, then delete.
- Delete removes:
  - archived rollout JSONL files
  - matching rows in `session_index.jsonl`
  - matching `shell_snapshots/<id>.sh`
- Delete ignores missing sidecars and never touches SQLite.

## Preview Behavior

Preview is loaded lazily and cached by `path + mtime`.

The rendered transcript is hybrid:

- user and assistant text are shown normally
- tool calls and outputs are summarized inline
- low-signal event and reasoning records are mostly suppressed

## Tests

Run the checks used during implementation:

```bash
make verify
```

For development helpers and command shortcuts, see [`Makefile`](./Makefile).
