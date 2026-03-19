import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";

import { parsePreviewFromFile, renderPreviewMarkdown, shortenJSON, truncate } from "../src/lib/preview";
import type { SessionRecord } from "../src/types";

const paths: string[] = [];

afterEach(async () => {
  await Promise.all(paths.splice(0).map((path) => rm(path, { recursive: true, force: true })));
});

describe("preview parser", () => {
  it("renders user, assistant, tool, and context blocks", async () => {
    const record = await makeRecord([
      { type: "event_msg", payload: { type: "user_message", message: "How do I archive this?" } },
      {
        type: "response_item",
        payload: { type: "message", role: "developer", content: [{ type: "input_text", text: "system context" }] },
      },
      {
        type: "response_item",
        payload: { type: "message", role: "assistant", content: [{ type: "output_text", text: "Use archive." }] },
      },
      { type: "response_item", payload: { type: "function_call", name: "exec_command", arguments: { cmd: "ls -la" } } },
      { type: "response_item", payload: { type: "function_call_output", call_id: "call-1", output: "done" } },
    ]);

    const document = await parsePreviewFromFile(record);

    expect(document.blocks.map((block) => block.title)).toEqual(["User", "Context", "Assistant", "exec_command", "call-1"]);

    const hidden = renderPreviewMarkdown(document, false);
    const shown = renderPreviewMarkdown(document, true);

    expect(hidden).not.toContain("system context");
    expect(shown).toContain("system context");
    expect(shown).toContain("Use archive.");
    expect(shown).toContain("call-1");
  });

  it("keeps low-signal event blocks that are intentionally surfaced", async () => {
    const record = await makeRecord([{ type: "event_msg", payload: { type: "turn_started" } }]);

    const document = await parsePreviewFromFile(record);

    expect(document.blocks).toHaveLength(1);
    expect(document.blocks[0]?.title).toBe("turn_started");
  });

  it("adds a no transcript block when no supported records exist", async () => {
    const record = await makeRecord([{ type: "session_meta", payload: { id: "ignored" } }]);

    const document = await parsePreviewFromFile(record);

    expect(document.blocks[0]?.title).toBe("No transcript");
  });
});

describe("preview helpers", () => {
  it("shortens JSON and text consistently", () => {
    expect(shortenJSON({ cmd: "ls -la" }, 12)).toContain("...");
    expect(truncate("abcdefghijklmnopqrstuvwxyz", 8)).toBe("abcde...");
  });
});

async function makeRecord(lines: unknown[]): Promise<SessionRecord> {
  const root = await mkdtemp(join(tmpdir(), "raycast-preview-"));
  paths.push(root);

  const id = "11111111-1111-1111-1111-111111111111";
  const path = join(root, `rollout-2026-03-19T10-42-03-${id}.jsonl`);
  await writeFile(path, `${lines.map((line) => JSON.stringify(line)).join("\n")}\n`, "utf8");

  return {
    id,
    path,
    status: "active",
    created_at: "2026-03-19T10:42:03Z",
    updated_at: "2026-03-19T10:42:03Z",
    cwd: "/tmp/app",
    title: "Preview Test",
    source: "vscode",
    child_count: 0,
    size_bytes: 10,
    has_preview: true,
  };
}
