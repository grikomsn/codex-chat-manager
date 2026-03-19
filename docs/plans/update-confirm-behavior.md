# Update Confirm Behavior (Cascade-Aware)

## Background

This project intentionally treats Codex rollout JSONL files as the source of truth. It also intentionally groups parent sessions with subagent children, and **parent actions cascade to grouped children**.

Today, some confirmation UIs describe only the *requested IDs* (often just the parent) even when the underlying operation will act on the parent **and** its children via `Store.ResolveTargets()`.

That mismatch is most visible when a user triggers an action on the currently highlighted parent without explicitly selecting the group:

- TUI: confirmation prompt can say “Archive 1 session(s)?” but the archive will move multiple rollout files.
- Raycast: confirmation prompt can say “Archive 1 Conversation?” while the CLI mutation expands to multiple session IDs.

## Goals

- Make confirmations **cascade-aware**: display the number of sessions that will be affected after `ResolveTargets()` expansion.
- Make destructive confirmations clearer:
  - explicitly mention that grouped child sessions are included when applicable
  - surface “delete is blocked by active sessions” *before* running the mutation when we can determine that from the snapshot
- Keep storage semantics unchanged (no new sidecars, no SQLite mutation, no changes to archive/unarchive/delete rules).

## Non-Goals

- Adding new “plan-only/dry-run” modes for mutations.
- Changing selection semantics (what IDs are requested vs. resolved).
- Changing the underlying cascade rules (parent cascades; child-only requests do not).

## Implementation Plan

### TUI (`internal/tui/`)

- Update `beginConfirm()` to:
  - compute `requestedIDs` (selection or highlighted parent)
  - call `m.store.ResolveTargets(requestedIDs)` to compute the *resolved* session set
  - build a confirmation title that uses the resolved count (and optionally shows a short ID preview)
  - for delete: if any resolved record is not archived, show an error and skip confirmation (mirrors `Store.Delete()` constraints)

Add a unit test proving that confirming an action on a highlighted parent with children shows the expanded session count.

### Raycast (`raycast/src/`)

- Update the confirm dialog copy in `SessionBrowser.runMutation()` to:
  - compute affected session IDs via `group.cascades_to`
  - display both “conversation(s)” and “session(s)” counts when they differ
  - mention child sessions are included when the cascade expands beyond the requested parent IDs

Prefer using the already-present `cascades_to` data rather than introducing an additional CLI round-trip.

### CLI (`internal/cli/`)

- Keep `sessions delete` requiring an explicit confirmation flag (`--yes`) for non-interactive callers (Raycast).
- Ensure flag/help text describes the behavior accurately (no “skip prompt” wording unless a prompt exists).

## Acceptance Checks

- TUI confirm prompt count matches the number of sessions `ResolveTargets()` will operate on for the same requested IDs.
- Raycast confirm dialog reflects the cascade size using `cascades_to`.
- No changes to archive/unarchive/delete filesystem behavior.

