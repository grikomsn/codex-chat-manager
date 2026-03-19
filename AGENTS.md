# AGENTS.md

Maintainer and coding-agent notes only. User-facing behavior belongs in `README.md`.

## Domain Rules

- Treat Codex rollout JSONL files as the only source of truth for inventory and destructive actions. `session_index.jsonl` is enrichment only.
- Do not add SQLite mutation or cleanup unless explicitly requested. This repo intentionally avoids it.
- Keep archive/unarchive behavior aligned with upstream Codex expectations: archive is a move into flat `archived_sessions`; unarchive restores into dated `sessions/YYYY/MM/DD` and bumps mtime.
- Keep delete conservative: active sessions must remain undeletable through normal flows.
- Parent session actions cascade to grouped subagent children by design. If you change grouping or selection semantics, update both CLI and TUI behavior together.
- The Codex on-disk format is reverse-engineered and unstable. Before changing storage behavior, verify against current upstream `openai/codex` rollout code/tests, not just local sample files.

## Build Commands

```bash
make build        # Build ./bin/codex-chat-manager with version ldflags
make test         # Run all tests
make test-race    # Run tests with race detection
make vet          # Run go vet
make verify       # Run test + vet + test-race (CI gate)
make check        # Run fmt + tidy + verify (pre-commit)
make completions  # Generate shell completions to ./completions/
```

### Running Single Tests

```bash
go test -v -run TestName ./internal/session/...
go test -v -run TestParseRolloutName ./internal/session/...
go test -v ./internal/cli/...  # Run all CLI tests
```

For TUI tests involving terminal output, run without `-v` to see rendered output.

## Code Style Guidelines

### Go

**Imports:** Group imports: standard library, external packages, internal packages. Separate groups with blank lines.

```go
import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"

	"github.com/grikomsn/codex-chat-manager/internal/session"
)
```

**Error Wrapping:** Always use `%w` verb for error wrapping to preserve the error chain:

```go
if err := os.Rename(src, dst); err != nil {
	return fmt.Errorf("archive %s: %w", record.ID, err)
}
```

**Constants First:** Define type-safe string constants at the top of files before types:

```go
type Status string

const (
	StatusActive   Status = "active"
	StatusArchived  Status = "archived"
)
```

**Cobra Commands:** Each command in its own file with `init()` function that registers flags and adds to parent. Use `RunE` for commands that return errors. Use `cobra.NoArgs` for commands that take no positional arguments.

**Tests:** Use `t.Parallel()` for unit tests. Use `t.Helper()` for test helper functions. Create fixtures in temp directories with `t.TempDir()`.

### TypeScript (Raycast)

**Imports:** Node built-ins with `node:` prefix, then external packages, then local imports.

**Error Classes:** Extend `Error` with custom properties for context:

```typescript
export class ManagerCommandError extends Error {
  command: string;
  constructor(message: string, command: string) {
    super(message);
    this.name = "ManagerCommandError";
    this.command = command;
  }
}
```

## Testing Patterns

### Go Test Fixtures

Create test fixtures using `t.TempDir()` and helper functions:

```go
func testStoreWithOneSession(t *testing.T, status Status) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	cfg := Config{
		CodexHome:   root,
		SessionsDir: filepath.Join(root, "sessions"),
		// ...
	}
	// Create test files...
	return NewStore(cfg), id
}
```

### JSONL Test Data

Use `session-meta` (hyphen, not underscore) in test fixtures to match upstream Codex format:

```go
body := `{"type":"session-meta","payload":{"id":"` + id + `","cwd":"/tmp"}}` + "\n"
```

## Architecture Conventions

**CLI Layer (`internal/cli/`):** Uses Cobra for command routing. One file per command with `init()` registration. Global flags (`--codex-home`) on root command. Command-specific flags in command files.

**Session Layer (`internal/session/`):** Pure Go, no IO in types. All filesystem operations through `Store` struct. `Config` struct centralizes all paths.

**TUI Layer (`internal/tui/`):** Bubble Tea model with `tea.Cmd` for async operations. State machine via `mode` constants. Key bindings in `keyMap` struct using `bubbles/key`.

**Raycast Layer (`raycast/src/`):** CLI wrapper that parses `--json` output. Uses `@raycast/utils` for async operations. Types in separate files.

## Key Files

| Path | Purpose |
|------|---------|
| `internal/cli/root.go` | Cobra root command, global flags, version |
| `internal/session/config.go` | Path resolution, CODEX_HOME override |
| `internal/session/types.go` | Domain types with JSON tags |
| `internal/session/actions.go` | Archive/unarchive/delete operations |
| `internal/session/discovery.go` | Rollout file scanning and grouping |
| `internal/tui/tui.go` | Bubble Tea model and keybindings |
| `raycast/src/cli-core.ts` | JSON parsing for CLI output |

## Version Injection

Version is injected at build time via ldflags:

```bash
go build -ldflags '-s -w -X github.com/grikomsn/codex-chat-manager/internal/cli.Version=v1.2.3' ./cmd/codex-chat-manager
```

The `Makefile` handles this automatically. Version defaults to `dev` if not set.