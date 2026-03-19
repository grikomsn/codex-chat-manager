import { describe, expect, it } from "vitest";

import { displayTitle, subtitle } from "../src/record";
import type { SessionRecord } from "../src/types";

function createRecord(overrides: Partial<SessionRecord> = {}): SessionRecord {
  return {
    id: "test-id",
    path: "/path/to/session",
    status: "active",
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    child_count: 0,
    size_bytes: 0,
    has_preview: false,
    ...overrides,
  };
}

describe("displayTitle", () => {
  it("returns record.title when present", () => {
    const record = createRecord({ title: "My Session Title" });
    expect(displayTitle(record)).toBe("My Session Title");
  });

  it("ignores empty title and falls back to cwd", () => {
    const record = createRecord({ title: "", cwd: "/path/to/project" });
    expect(displayTitle(record)).toBe("project");
  });

  describe("falls back to cwd basename when no title", () => {
    it.each([
      ["/path/to/project", "project"],
      ["/Users/foo/my-app", "my-app"],
      ["/home/user/workspace/codex-session", "codex-session"],
    ])("cwd %s -> %s", (cwd, expected) => {
      const record = createRecord({ cwd });
      expect(displayTitle(record)).toBe(expected);
    });
  });

  it("falls back to record.id when no cwd", () => {
    const record = createRecord({ id: "session-123" });
    expect(displayTitle(record)).toBe("session-123");
  });

  describe("handles cwd edge cases", () => {
    it("handles cwd with trailing slashes", () => {
      const record = createRecord({ cwd: "/path/to/project///" });
      expect(displayTitle(record)).toBe("project");
    });

    it("handles cwd that is just '/'", () => {
      const record = createRecord({ cwd: "/" });
      expect(displayTitle(record)).toBe("/");
    });

    it("handles cwd with multiple consecutive slashes", () => {
      const record = createRecord({ cwd: "/path//to///project" });
      expect(displayTitle(record)).toBe("project");
    });
  });
});

describe("subtitle", () => {
  describe("returns cwd basename with agent info", () => {
    it.each([
      ["/path/to/project", "codex", undefined, "project (codex)"],
      ["/Users/dev/my-app", undefined, "assistant", "my-app (assistant)"],
      ["/home/workspace/test", "agent-1", "planner", "test (agent-1)"],
    ])(
      "cwd=%s, nickname=%s, role=%s -> %s",
      (cwd, nickname, role, expected) => {
        const record = createRecord({
          cwd,
          agent_nickname: nickname,
          agent_role: role,
        });
        expect(subtitle(record)).toBe(expected);
      }
    );
  });

  it("returns cwd basename when no agent info", () => {
    const record = createRecord({ cwd: "/path/to/project" });
    expect(subtitle(record)).toBe("project");
  });

  it("returns 'unknown cwd' when no cwd", () => {
    const record = createRecord();
    expect(subtitle(record)).toBe("unknown cwd");
  });

  it("prefers agent_nickname over agent_role", () => {
    const record = createRecord({
      cwd: "/path/to/app",
      agent_nickname: "primary",
      agent_role: "assistant",
    });
    expect(subtitle(record)).toBe("app (primary)");
  });

  describe("handles whitespace in cwd", () => {
    it.each([
      ["/path/to/project  ", "project"],
      ["  /path/to/my-folder", "my-folder"],
      ["/path  /to  /folder", "folder"],
    ])("cwd='%s' -> basename '%s'", (cwd, expected) => {
      const record = createRecord({ cwd });
      expect(subtitle(record)).toBe(expected);
    });
  });
});
