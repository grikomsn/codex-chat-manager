import { describe, expect, it } from "vitest";

import {
  formatManagerResponseErrorMessage,
  ManagerResponseError,
  parseManagerJSON,
  parseManagerResponse,
  ParserError,
} from "../src/cli-core";

const groupedSessionEnvelope = JSON.stringify({
  schema_version: "1",
  command: "sessions list",
  ok: true,
  data: [
    {
      parent: {
        id: "parent-1",
        path: "/tmp/rollout-parent.jsonl",
        status: "active",
        created_at: "2026-03-19T10:42:03Z",
        updated_at: "2026-03-19T11:00:00Z",
        cwd: "/tmp/project",
        child_count: 1,
        size_bytes: 100,
        has_preview: true,
      },
      children: [
        {
          id: "child-1",
          path: "/tmp/rollout-child.jsonl",
          status: "archived",
          created_at: "2026-03-19T10:43:03Z",
          updated_at: "2026-03-19T12:00:00Z",
          cwd: "/tmp/project",
          parent_id: "parent-1",
          child_count: 0,
          size_bytes: 50,
          has_preview: true,
        },
      ],
      status: "mixed",
      aggregate_at: "2026-03-19T12:00:00Z",
      child_count: 1,
      cascades_to: ["parent-1", "child-1"],
      parent_exists: true,
    },
  ],
});

const structuredErrorEnvelope = JSON.stringify({
  schema_version: "1",
  command: "sessions delete",
  ok: false,
  error: {
    code: "delete_blocked_active",
    message: "delete blocked by active sessions: parent-1",
    details: {
      type: "delete",
      requested_ids: ["parent-1"],
      blocked_by_active_ids: ["parent-1"],
      targets: [],
      skipped: [],
    },
  },
});

describe("cli-core contract", () => {
  it("unwraps successful envelopes", () => {
    const response = parseManagerResponse<{ status: string }[]>(
      groupedSessionEnvelope,
    );

    expect(response.envelope?.ok).toBe(true);
    expect(response.data[0]?.status).toBe("mixed");
    expect("mixed_status" in (response.data[0] as Record<string, unknown>)).toBe(
      false,
    );
  });

  it("parses grouped payloads through parseManagerJSON", () => {
    const groups = parseManagerJSON<{ parent: { id: string }; status: string }[]>(
      groupedSessionEnvelope,
    );

    expect(groups).toHaveLength(1);
    expect(groups[0]).toMatchObject({
      parent: { id: "parent-1" },
      status: "mixed",
    });
  });

  it("throws ManagerResponseError for structured failures", () => {
    expect(() => parseManagerJSON(structuredErrorEnvelope)).toThrow(
      ManagerResponseError,
    );

    try {
      parseManagerJSON(structuredErrorEnvelope);
      expect.fail("Should have thrown");
    } catch (error) {
      expect(error).toBeInstanceOf(ManagerResponseError);
      const responseError = error as ManagerResponseError;
      expect(responseError.code).toBe("delete_blocked_active");
      expect(responseError.blockedIds).toEqual(["parent-1"]);
      expect(formatManagerResponseErrorMessage(responseError.response)).toContain(
        "blocked IDs: parent-1",
      );
    }
  });

  it("rejects empty output", () => {
    expect(() => parseManagerJSON("")).toThrow(
      "Empty response from codex-chat-manager",
    );
  });

  it("rejects malformed output with ParserError", () => {
    expect(() => parseManagerJSON("{oops")).toThrow(ParserError);
  });
});
