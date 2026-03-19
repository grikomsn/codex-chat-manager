# codex-chat-manager

CLI, TUI, and Raycast extension for managing local OpenAI Codex chat sessions.

The app reads Codex rollout files directly from disk and treats them as the source of truth. It supports active and archived sessions, parent/subagent grouping, transcript preview, archive/unarchive, resume, bulk delete, and a Raycast browser that reuses the CLI JSON output for discovery and mutations.

## Install

The recommended path is the release installer:

```bash
curl -fsSL https://raw.githubusercontent.com/grikomsn/codex-chat-manager/main/install.sh | bash
```

By default it installs `codex-chat-manager` into `~/.local/bin`, then falls back to `~/bin` if needed. Make sure that directory is on your `PATH`:

```bash
export PATH="$HOME/.local/bin:$HOME/bin:$PATH"
```

You can also download a release asset manually from [GitHub Releases](https://github.com/grikomsn/codex-chat-manager/releases). Published archives follow this naming scheme:

- `codex-chat-manager_<tag>_darwin_amd64.tar.gz`
- `codex-chat-manager_<tag>_darwin_arm64.tar.gz`
- `codex-chat-manager_<tag>_linux_amd64.tar.gz`
- `codex-chat-manager_<tag>_linux_arm64.tar.gz`
- `codex-chat-manager_<tag>_windows_amd64.zip`
- `codex-chat-manager_<tag>_windows_arm64.zip`

Each release also includes `checksums.txt`.

## Usage

Run the TUI:

```bash
codex-chat-manager tui
codex-chat-manager tui --codex-home /path/to/codex-home
```

List sessions:

```bash
codex-chat-manager sessions list
codex-chat-manager sessions list --status archived --filter bundle
codex-chat-manager sessions list --json --include-children
```

Archive or unarchive:

```bash
codex-chat-manager sessions archive --id SESSION_ID
codex-chat-manager sessions unarchive --id SESSION_ID
```

Delete archived sessions:

```bash
codex-chat-manager sessions delete --id SESSION_ID --yes
```

Resume an active session:

```bash
codex-chat-manager sessions resume --id SESSION_ID
```

The app can operate on a copied Codex home if you want to inspect or test against a safe fixture instead of your live `~/.codex`.

## Raycast Extension

This repo also ships a standalone Raycast workspace in [`raycast`](./raycast).

The extension is intentionally layered on top of the existing Go app:

- it uses `codex-chat-manager sessions list --json` for discovery
- it uses `codex-chat-manager sessions archive|unarchive|delete --json` for mutations
- it parses rollout JSONL files locally for transcript preview
- it does not change CLI or TUI storage semantics

Current Raycast commands:

- `Conversations`
- `Active Conversations`
- `Archived Conversations`

Raycast preferences:

- `managerBinaryPath`: optional override for the `codex-chat-manager` binary
- `codexHome`: optional override for `CODEX_HOME`

## Why

Codex stores valuable local session history, but the built-in storage is file-oriented and not optimized for bulk management. This project provides a focused local manager for browsing, previewing, grouping subagent threads, and performing safe archive or delete workflows on top of the rollout files Codex already writes.

## Prerequisites

- A local Codex installation that writes session files under `CODEX_HOME`
- A session tree that matches current Codex rollout conventions such as `sessions/YYYY/MM/DD/rollout-...jsonl` and `archived_sessions/rollout-...jsonl`
- For Raycast development: Node `24.13.0` and npm `11.12.0` or newer, plus a local Raycast installation on macOS

## Storage Model

By default the app uses `~/.codex`.

It reads:

- `~/.codex/sessions/YYYY/MM/DD/rollout-...jsonl`
- `~/.codex/archived_sessions/rollout-...jsonl`
- `~/.codex/session_index.jsonl` as a best-effort title cache only

It does not use SQLite as the source of truth and does not mutate SQLite on delete.

The Codex on-disk format is reverse-engineered from upstream behavior and may drift over time. The tool intentionally treats rollout files as canonical, `session_index.jsonl` as advisory only, and SQLite as out of scope.

## Safety Rules

- Sessions are discovered from rollout files, not from `session_index.jsonl`.
- Parent session actions cascade to grouped subagent children.
- Deleting active sessions is blocked. Archive first, then delete.
- Delete removes:
  - archived rollout JSONL files
  - matching rows in `session_index.jsonl`
  - matching `shell_snapshots/<id>.sh`
- Delete ignores missing sidecars and never touches SQLite.

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

## Preview Behavior

Preview is loaded lazily and cached by `path + mtime`.

The rendered transcript is hybrid:

- user and assistant text are shown normally
- tool calls and outputs are summarized inline
- low-signal event and reasoning records are mostly suppressed

## Build From Source

```bash
go build ./cmd/codex-chat-manager
./codex-chat-manager tui
```

If you prefer `go install`, you can still build from source that way as well:

```bash
go install github.com/grikomsn/codex-chat-manager/cmd/codex-chat-manager@latest
```

Build the Raycast extension workspace:

```bash
cd raycast
npm install
npm run build
```

## Tests

Run the checks used during implementation:

```bash
make verify
```

For development helpers and command shortcuts, see [`Makefile`](./Makefile).

For the Raycast workspace:

```bash
cd raycast
npm test
npm run lint
npm run build
```

## License

MIT License. See [`LICENSE`](./LICENSE) for the full text.
