# AGENTS.md

Maintainer and coding-agent notes only. User-facing behavior belongs in `README.md`.

- Treat Codex rollout JSONL files as the only source of truth for inventory and destructive actions. `session_index.jsonl` is enrichment only.
- Do not add SQLite mutation or cleanup unless explicitly requested. This repo intentionally avoids it.
- Keep archive/unarchive behavior aligned with upstream Codex expectations: archive is a move into flat `archived_sessions`; unarchive restores into dated `sessions/YYYY/MM/DD` and bumps mtime.
- Keep delete conservative: active sessions must remain undeletable through normal flows.
- Parent session actions cascade to grouped subagent children by design. If you change grouping or selection semantics, update both CLI and TUI behavior together.
- The Codex on-disk format is reverse-engineered and unstable. Before changing storage behavior, verify against current upstream `openai/codex` rollout code/tests, not just local sample files.
