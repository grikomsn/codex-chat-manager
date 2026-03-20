import { describe, expect, it } from "vitest";

import { managerCommandErrorFromOutput, ManagerCommandError } from "../src/cli";

const successEnvelope = JSON.stringify({
  schema_version: "1",
  command: "sessions list",
  ok: true,
  data: [
    {
      parent: { id: "group-1" },
      status: "mixed",
      aggregate_at: "2026-03-19T12:00:00Z",
      child_count: 1,
      cascades_to: ["group-1", "child-1"],
      parent_exists: true,
    },
  ],
});

const blockedDeleteEnvelope = JSON.stringify({
  schema_version: "1",
  command: "sessions delete",
  ok: false,
  error: {
    code: "delete_blocked_active",
    message: "delete blocked by active sessions: group-1",
    details: {
      type: "delete",
      requested_ids: ["group-1"],
      blocked_by_active_ids: ["group-1"],
      targets: [],
      skipped: [],
    },
  },
});

describe("cli runtime helpers", () => {
  it("returns undefined for successful envelopes", () => {
    expect(
      managerCommandErrorFromOutput(successEnvelope, "codex-chat-manager"),
    ).toBeUndefined();
  });

  it("builds rich ManagerCommandError from structured failures", () => {
    const error = managerCommandErrorFromOutput(
      blockedDeleteEnvelope,
      "codex-chat-manager sessions delete --id group-1 --yes --json",
      {
        stderr: "",
        exitCode: 1,
      },
    );

    expect(error).toBeInstanceOf(ManagerCommandError);
    expect(error).toMatchObject({
      command: "codex-chat-manager sessions delete --id group-1 --yes --json",
      code: "delete_blocked_active",
      blockedIds: ["group-1"],
      exitCode: 1,
    });
    expect(error?.message).toContain("blocked IDs: group-1");
  });

  it("returns undefined for malformed output", () => {
    expect(
      managerCommandErrorFromOutput("not valid json", "codex-chat-manager"),
    ).toBeUndefined();
  });

  it("returns undefined for empty output", () => {
    expect(
      managerCommandErrorFromOutput("", "codex-chat-manager"),
    ).toBeUndefined();
  });
});
