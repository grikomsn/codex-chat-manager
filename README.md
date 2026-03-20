# codex-chat-manager

CLI, TUI, and Raycast extension for managing local OpenAI Codex chat sessions.

The app reads Codex rollout files directly from disk and treats them as the source of truth. It supports active and archived sessions, parent/subagent grouping, transcript preview, archive/unarchive, resume, bulk delete, and a Raycast browser that reuses the CLI JSON output for discovery and mutations.

> [!WARNING]
> This project is in early stages of development. APIs, commands, and storage behavior may change without notice. Use with caution in production environments.

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
codex-chat-manager sessions resume --id SESSION_ID --json
codex-chat-manager sessions resume --id SESSION_ID --print-cmd
```

Shell completions:

```bash
codex-chat-manager completion bash > /etc/bash_completion.d/codex-chat-manager
codex-chat-manager completion zsh > "${fpath[1]}/_codex-chat-manager"
codex-chat-manager completion fish > ~/.config/fish/completions/codex-chat-manager.fish
```

Global flags:

```bash
codex-chat-manager --version           # Print version
codex-chat-manager --verbose ...       # Enable verbose output
codex-chat-manager --help              # Show help
```

The app can operate on a copied Codex home if you want to inspect or test against a safe fixture instead of your live `~/.codex`.

## JSON Contract

When `--json` is enabled, the CLI returns a stable machine-readable envelope instead of raw domain objects. The top-level response always includes `schema_version`, `command`, `ok`, and either `data` or a structured `error`.

Agents and integrations should rely on `schema_version` before decoding `data`, since the nested payload may evolve independently of the envelope.

- `schema_version`: stable envelope version, currently `"1"`
- `command`: command path such as `sessions list` or `sessions delete`
- `ok`: `true` for successful responses, `false` for structured command failures
- `data`: nested command payload on success
- `error.code`: stable machine-readable failure code such as `invalid_request`, `inventory_unavailable`, `operation_failed`, or `delete_blocked_active`
- `error.message`: human-readable error text when `ok` is `false`
- `error.details`: optional structured failure details, including partial `ActionPlan` data for mutation failures when it is safe to expose

Example `sessions list` response:

```json
{
  "schema_version": "1",
  "command": "sessions list",
  "ok": true,
  "data": [
    {
      "parent": {
        "id": "11111111-1111-1111-1111-111111111111",
        "path": "/Users/me/.codex/sessions/2026/03/19/rollout-...jsonl",
        "status": "active"
      },
      "status": "active",
      "aggregate_at": "2026-03-19T10:42:03Z",
      "child_count": 0,
      "cascades_to": [
        "11111111-1111-1111-1111-111111111111"
      ],
      "parent_exists": true
    }
  ]
}
```

Example mutation response:

```json
{
  "schema_version": "1",
  "command": "sessions archive",
  "ok": true,
  "data": {
    "type": "archive",
    "requested_ids": [
      "11111111-1111-1111-1111-111111111111"
    ],
    "target_ids": [
      "11111111-1111-1111-1111-111111111111"
    ],
    "targets": [
      {
        "id": "11111111-1111-1111-1111-111111111111",
        "path": "/Users/me/.codex/archived_sessions/rollout-...jsonl",
        "status": "archived",
        "is_child": false
      }
    ],
    "skipped": []
  }
}
```

Example mutation failure response:

```json
{
  "schema_version": "1",
  "command": "sessions delete",
  "ok": false,
  "error": {
    "code": "delete_blocked_active",
    "message": "delete blocked by active sessions: 11111111-1111-1111-1111-111111111111",
    "details": {
      "type": "delete",
      "requested_ids": [
        "11111111-1111-1111-1111-111111111111"
      ],
      "blocked_by_active_ids": [
        "11111111-1111-1111-1111-111111111111"
      ],
      "targets": [],
      "skipped": []
    }
  }
}
```

Example `sessions resume --json` response:

```json
{
  "schema_version": "1",
  "command": "sessions resume",
  "ok": true,
  "data": {
    "intent": {
      "requested_id": "11111111-1111-1111-1111-111111111111",
      "session_id": "11111111-1111-1111-1111-111111111111",
      "status": "active",
      "eligible": true,
      "working_directory": "/Users/me/project",
      "executable": "codex",
      "args": [
        "resume",
        "11111111-1111-1111-1111-111111111111"
      ],
      "env_overrides": {
        "CODEX_HOME": "/Users/me/.codex"
      }
    },
    "executed": false
  }
}
```

Agents can query resume intent before execution:

```bash
codex-chat-manager sessions resume --id SESSION_ID --json
```

If the returned intent is eligible and the automation wants to proceed, it can opt into execution explicitly:

```bash
codex-chat-manager sessions resume --id SESSION_ID --execute --json
```

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
- `/`: enter filter mode (type to filter, Enter to apply, Esc to cancel)
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
