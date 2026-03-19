import { Color, Icon } from "@raycast/api";
import { describe, expect, it } from "vitest";

import {
  canDeleteGroups,
  childGroups,
  countRenderableBlocks,
  formatDate,
  groupKeywords,
  shouldAllowArchive,
  shouldAllowUnarchive,
  statusIcon,
  statusLabel,
} from "../src/format";
import type { PreviewBlock, SessionGroup, SessionRecord } from "../src/types";

function createMockRecord(overrides: Partial<SessionRecord> = {}): SessionRecord {
  return {
    id: "test-id",
    path: "/path/to/session",
    status: "active",
    created_at: "2024-01-15T10:30:00Z",
    updated_at: "2024-01-15T12:00:00Z",
    cwd: "/home/user/project",
    title: "Test Session",
    source: "cli",
    agent_nickname: "assistant",
    agent_role: "default",
    child_count: 0,
    size_bytes: 1024,
    has_preview: true,
    ...overrides,
  };
}

function createMockGroup(overrides: Partial<SessionGroup> = {}): SessionGroup {
  return {
    parent: createMockRecord(),
    status: "active",
    aggregate_at: "2024-01-15T12:00:00Z",
    mixed_status: false,
    child_count: 0,
    cascades_to: ["test-id"],
    parent_exists: true,
    ...overrides,
  };
}

describe("formatDate", () => {
  it("formats valid ISO date strings", () => {
    const result = formatDate("2024-01-15T10:30:00Z");
    expect(result).toMatch(/2024/);
    expect(result).toMatch(/Jan/);
    expect(result).toMatch(/15/);
  });

  it("handles invalid dates gracefully", () => {
    expect(() => formatDate("invalid-date")).toThrow(RangeError);
  });

  it("formats date with numeric year, short month, and 2-digit day/time", () => {
    const result = formatDate("2024-12-25T00:00:00Z");
    expect(result).toMatch(/2024/);
    expect(result).toMatch(/Dec/);
  });
});

describe("statusLabel", () => {
  it.each([
    ["active", false, "active"],
    ["archived", false, "archived"],
    ["mixed", false, "mixed"],
    ["active", true, "mixed"],
    ["archived", true, "mixed"],
    ["mixed", true, "mixed"],
  ] as const)(
    "returns '%s' when status is '%s' and mixedStatus is %s",
    (status, mixedStatus, expected) => {
      expect(statusLabel(status, mixedStatus)).toBe(expected);
    },
  );

  it("returns status as-is when mixedStatus is false", () => {
    expect(statusLabel("active", false)).toBe("active");
    expect(statusLabel("archived", false)).toBe("archived");
    expect(statusLabel("mixed", false)).toBe("mixed");
  });

  it("returns 'mixed' when mixedStatus is true", () => {
    expect(statusLabel("active", true)).toBe("mixed");
    expect(statusLabel("archived", true)).toBe("mixed");
  });
});

describe("statusIcon", () => {
  it("returns yellow circle for mixed status", () => {
    const result = statusIcon("mixed", false);
    expect(result).toEqual({ source: Icon.CircleFilled, tintColor: Color.Yellow });
  });

  it("returns yellow circle when mixedStatus is true", () => {
    const result = statusIcon("active", true);
    expect(result).toEqual({ source: Icon.CircleFilled, tintColor: Color.Yellow });
  });

  it("returns gray circle for archived status", () => {
    const result = statusIcon("archived", false);
    expect(result).toEqual({ source: Icon.Circle, tintColor: Color.SecondaryText });
  });

  it("returns green circle for active status", () => {
    const result = statusIcon("active", false);
    expect(result).toEqual({ source: Icon.CircleFilled, tintColor: Color.Green });
  });

  it("prioritizes mixedStatus over status value", () => {
    const result = statusIcon("archived", true);
    expect(result).toEqual({ source: Icon.CircleFilled, tintColor: Color.Yellow });
  });
});

describe("groupKeywords", () => {
  it("returns array of string values from group", () => {
    const group = createMockGroup();
    const keywords = groupKeywords(group);
    expect(keywords).toContain("test-id");
    expect(keywords).toContain("Test Session");
    expect(keywords).toContain("/home/user/project");
    expect(keywords).toContain("cli");
    expect(keywords).toContain("assistant");
    expect(keywords).toContain("default");
  });

  it("filters out undefined/null values", () => {
    const group = createMockGroup({
      parent: createMockRecord({
        title: undefined,
        source: undefined,
        agent_nickname: undefined,
        agent_role: undefined,
      }),
    });
    const keywords = groupKeywords(group);
    expect(keywords).not.toContain(undefined);
    expect(keywords).not.toContain(null);
    expect(keywords).toContain("test-id");
    expect(keywords).toContain("/home/user/project");
  });

  it("includes parent and children fields", () => {
    const group = createMockGroup({
      children: [
        createMockRecord({
          id: "child-1",
          title: "Child Session",
          cwd: "/home/user/child-project",
          agent_nickname: "builder",
        }),
      ],
    });
    const keywords = groupKeywords(group);
    expect(keywords).toContain("test-id");
    expect(keywords).toContain("child-1");
    expect(keywords).toContain("Child Session");
    expect(keywords).toContain("/home/user/child-project");
    expect(keywords).toContain("builder");
  });

  it("uses displayTitle for titles", () => {
    const group = createMockGroup({
      parent: createMockRecord({
        id: "parent-id",
        title: undefined,
        cwd: "/home/user/my-project",
      }),
    });
    const keywords = groupKeywords(group);
    expect(keywords).toContain("parent-id");
    expect(keywords).toContain("my-project");
  });
});

describe("childGroups", () => {
  it("transforms children array into SessionGroup array", () => {
    const group = createMockGroup({
      children: [
        createMockRecord({
          id: "child-1",
          status: "active",
          updated_at: "2024-01-15T14:00:00Z",
        }),
        createMockRecord({
          id: "child-2",
          status: "archived",
          updated_at: "2024-01-15T15:00:00Z",
        }),
      ],
    });
    const children = childGroups(group);
    expect(children).toHaveLength(2);
    expect(children[0].parent.id).toBe("child-1");
    expect(children[1].parent.id).toBe("child-2");
  });

  it("sets empty children array", () => {
    const group = createMockGroup({
      children: [createMockRecord({ id: "child-1" })],
    });
    const children = childGroups(group);
    expect(children[0].children).toEqual([]);
  });

  it("sets cascades_to to single element array with child id", () => {
    const group = createMockGroup({
      children: [createMockRecord({ id: "child-1" })],
    });
    const children = childGroups(group);
    expect(children[0].cascades_to).toEqual(["child-1"]);
  });

  it("sets correct status from child", () => {
    const group = createMockGroup({
      children: [
        createMockRecord({ id: "child-1", status: "archived" }),
        createMockRecord({ id: "child-2", status: "active" }),
      ],
    });
    const children = childGroups(group);
    expect(children[0].status).toBe("archived");
    expect(children[1].status).toBe("active");
  });

  it("returns empty array when no children", () => {
    const group = createMockGroup({ children: undefined });
    expect(childGroups(group)).toEqual([]);
  });
});

describe("canDeleteGroups", () => {
  it("returns true only when all groups are archived and not mixed", () => {
    const groups = [
      createMockGroup({ status: "archived", mixed_status: false }),
      createMockGroup({ status: "archived", mixed_status: false }),
    ];
    expect(canDeleteGroups(groups)).toBe(true);
  });

  it("returns false for empty array", () => {
    expect(canDeleteGroups([])).toBe(false);
  });

  it("returns false if any group has mixed_status", () => {
    const groups = [
      createMockGroup({ status: "archived", mixed_status: false }),
      createMockGroup({ status: "archived", mixed_status: true }),
    ];
    expect(canDeleteGroups(groups)).toBe(false);
  });

  it("returns false if any group is active", () => {
    const groups = [
      createMockGroup({ status: "archived", mixed_status: false }),
      createMockGroup({ status: "active", mixed_status: false }),
    ];
    expect(canDeleteGroups(groups)).toBe(false);
  });

  it("returns false if all groups are active", () => {
    const groups = [
      createMockGroup({ status: "active", mixed_status: false }),
    ];
    expect(canDeleteGroups(groups)).toBe(false);
  });

  it("returns false if any group is mixed status", () => {
    const groups = [
      createMockGroup({ status: "mixed", mixed_status: false }),
    ];
    expect(canDeleteGroups(groups)).toBe(false);
  });
});

describe("shouldAllowArchive", () => {
  it("returns true for active groups", () => {
    const group = createMockGroup({ status: "active", mixed_status: false });
    expect(shouldAllowArchive(group)).toBe(true);
  });

  it("returns true for mixed groups regardless of status", () => {
    const activeMixed = createMockGroup({ status: "active", mixed_status: true });
    const archivedMixed = createMockGroup({ status: "archived", mixed_status: true });
    expect(shouldAllowArchive(activeMixed)).toBe(true);
    expect(shouldAllowArchive(archivedMixed)).toBe(true);
  });

  it("returns false for non-mixed archived groups", () => {
    const group = createMockGroup({ status: "archived", mixed_status: false });
    expect(shouldAllowArchive(group)).toBe(false);
  });

  it.each([
    ["active", false, true],
    ["archived", false, false],
    ["mixed", false, true],
    ["active", true, true],
    ["archived", true, true],
    ["mixed", true, true],
  ] as const)(
    "returns %s for status '%s' with mixed_status %s",
    (status, mixedStatus, expected) => {
      const group = createMockGroup({ status, mixed_status: mixedStatus });
      expect(shouldAllowArchive(group)).toBe(expected);
    },
  );
});

describe("shouldAllowUnarchive", () => {
  it("returns true for archived groups", () => {
    const group = createMockGroup({ status: "archived", mixed_status: false });
    expect(shouldAllowUnarchive(group)).toBe(true);
  });

  it("returns true for mixed groups regardless of status", () => {
    const activeMixed = createMockGroup({ status: "active", mixed_status: true });
    const archivedMixed = createMockGroup({ status: "archived", mixed_status: true });
    expect(shouldAllowUnarchive(activeMixed)).toBe(true);
    expect(shouldAllowUnarchive(archivedMixed)).toBe(true);
  });

  it("returns false for non-mixed active groups", () => {
    const group = createMockGroup({ status: "active", mixed_status: false });
    expect(shouldAllowUnarchive(group)).toBe(false);
  });

  it.each([
    ["active", false, false],
    ["archived", false, true],
    ["mixed", false, true],
    ["active", true, true],
    ["archived", true, true],
    ["mixed", true, true],
  ] as const)(
    "returns %s for status '%s' with mixed_status %s",
    (status, mixedStatus, expected) => {
      const group = createMockGroup({ status, mixed_status: mixedStatus });
      expect(shouldAllowUnarchive(group)).toBe(expected);
    },
  );
});

describe("countRenderableBlocks", () => {
  function createBlock(title: string): PreviewBlock {
    return { kind: "user", title, body: "test body" };
  }

  it("returns total count when showSystem is true", () => {
    const blocks: PreviewBlock[] = [
      createBlock("Context"),
      createBlock("User Message"),
      createBlock("Assistant"),
    ];
    expect(countRenderableBlocks(blocks, true)).toBe(3);
  });

  it("returns count excluding Context blocks when showSystem is false", () => {
    const blocks: PreviewBlock[] = [
      createBlock("Context"),
      createBlock("User Message"),
      createBlock("Assistant"),
    ];
    expect(countRenderableBlocks(blocks, false)).toBe(2);
  });

  it("returns zero for empty array", () => {
    expect(countRenderableBlocks([], true)).toBe(0);
    expect(countRenderableBlocks([], false)).toBe(0);
  });

  it("excludes all Context blocks when showSystem is false", () => {
    const blocks: PreviewBlock[] = [
      createBlock("Context"),
      createBlock("Context"),
      createBlock("User Message"),
    ];
    expect(countRenderableBlocks(blocks, false)).toBe(1);
  });

  it("handles blocks without title", () => {
    const blocks: PreviewBlock[] = [
      { kind: "user", body: "no title" },
      { kind: "assistant", title: "Response", body: "text" },
    ];
    expect(countRenderableBlocks(blocks, false)).toBe(2);
  });

  it("only filters Context title, not other blocks", () => {
    const blocks: PreviewBlock[] = [
      createBlock("Context"),
      createBlock("User"),
      createBlock("Tool"),
      createBlock("Output"),
    ];
    expect(countRenderableBlocks(blocks, false)).toBe(3);
  });
});
