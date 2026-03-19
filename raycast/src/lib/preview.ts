import { createReadStream } from "node:fs";
import { stat } from "node:fs/promises";
import readline from "node:readline";

import { displayTitle } from "../record";
import type { PreviewBlock, PreviewDocument, SessionRecord } from "../types";

interface RecordEnvelope {
  type?: string;
  payload?: unknown;
}

interface EventPayload {
  type?: string;
  message?: string;
}

interface MessagePart {
  type?: string;
  text?: string;
}

interface OutputMessagePayload {
  type?: string;
  role?: string;
  content?: MessagePart[];
  arguments?: unknown;
  call_id?: string;
  name?: string;
  output?: string;
}

export async function parsePreviewFromFile(
  record: SessionRecord,
): Promise<PreviewDocument> {
  await stat(record.path);

  const document: PreviewDocument = {
    sessionId: record.id,
    title: displayTitle(record),
    blocks: [],
  };

  const reader = readline.createInterface({
    input: createReadStream(record.path, { encoding: "utf8" }),
    crlfDelay: Number.POSITIVE_INFINITY,
  });

  for await (const line of reader) {
    const trimmed = line.trim();
    if (!trimmed) {
      continue;
    }

    let envelope: RecordEnvelope;
    try {
      envelope = JSON.parse(trimmed) as RecordEnvelope;
    } catch (error) {
      if (process.env.DEBUG_PREVIEW) {
        console.error(`[preview] Skipping malformed line in ${record.path}: ${error instanceof Error ? error.message : String(error)}`);
      }
      continue;
    }

    if (envelope.type === "event_msg") {
      const block = eventBlock(envelope.payload);
      if (block) {
        document.blocks.push(block);
      }
      continue;
    }

    if (envelope.type === "response_item") {
      document.blocks.push(...responseBlocks(envelope.payload));
    }
  }

  if (document.blocks.length === 0) {
    document.blocks.push({
      kind: "event",
      title: "No transcript",
      body: "This session has no renderable user or assistant messages yet.",
    });
  }

  return document;
}

export function renderPreviewMarkdown(
  document: PreviewDocument,
  showSystem: boolean,
): string {
  const sections = [`# ${escapeMarkdown(document.title)}`];

  for (const block of document.blocks) {
    if (!showSystem && block.title === "Context") {
      continue;
    }

    const header = escapeMarkdown(block.title?.trim() || block.kind);
    const body = block.body?.trim();

    if (!body) {
      sections.push(`## ${header}`);
      continue;
    }

    if (block.kind === "tool_call" || block.kind === "tool_output") {
      sections.push(`## ${header}`, "```text", body, "```");
      continue;
    }

    sections.push(`## ${header}`, escapeMarkdown(body));
  }

  return sections.join("\n\n");
}

export function truncate(value: string, limit: number): string {
  const trimmed = value.trim();
  if (trimmed.length <= limit) {
    return trimmed;
  }
  if (limit <= 3) {
    return trimmed.slice(0, limit);
  }
  return `${trimmed.slice(0, limit - 3)}...`;
}

export function shortenJSON(value: unknown, limit: number): string {
  if (value === undefined || value === null) {
    return "";
  }

  try {
    return truncate(JSON.stringify(value, null, 2), limit);
  } catch {
    return truncate(String(value), limit);
  }
}

function eventBlock(payload: unknown): PreviewBlock | null {
  const event = payload as EventPayload | undefined;
  switch (event?.type) {
    case "user_message":
      return event.message?.trim()
        ? { kind: "user", title: "User", body: event.message }
        : null;
    case "agent_message":
      return event.message?.trim()
        ? { kind: "event", title: "Agent", body: event.message }
        : null;
    case "task_started":
    case "turn/completed":
    case "turn_started":
      return { kind: "event", title: event.type };
    default:
      return null;
  }
}

function responseBlocks(payload: unknown): PreviewBlock[] {
  const item = payload as OutputMessagePayload | undefined;
  switch (item?.type) {
    case "message": {
      const body = collectMessageText(item.content ?? []);
      if (!body) {
        return [];
      }
      if (item.role === "developer" || item.role === "system") {
        return [{ kind: "event", title: "Context", body }];
      }
      return [{ kind: "assistant", title: "Assistant", body }];
    }
    case "function_call":
      return [
        {
          kind: "tool_call",
          title: item.name || "Tool Call",
          body: shortenJSON(item.arguments, 200),
        },
      ];
    case "function_call_output":
      return [
        {
          kind: "tool_output",
          title: item.call_id || "Tool Output",
          body: truncate(item.output?.trim() || "command completed", 500),
        },
      ];
    default:
      return [];
  }
}

function collectMessageText(parts: MessagePart[]): string {
  return parts
    .filter(
      (part) =>
        part.text &&
        (part.type === "input_text" ||
          part.type === "output_text" ||
          part.type === "text"),
    )
    .map((part) => part.text?.trim() || "")
    .filter(Boolean)
    .join("\n")
    .trim();
}

function escapeMarkdown(value: string): string {
  return value.replace(/\r\n/g, "\n").replace(/\r/g, "\n").trim();
}
