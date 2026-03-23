import { createReadStream } from "node:fs";
import { stat } from "node:fs/promises";
import readline from "node:readline";

import { displayTitle } from "../record";
import type { PreviewBlock, PreviewDocument, SessionRecord } from "../types";

const INJECTED_AGENTS_CONTEXT_LIMIT = 2500;

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
  input?: unknown;
  status?: string;
  call_id?: string;
  name?: string;
  output?: string;
}

function cmdFromToolArguments(value: unknown): string {
  if (!value) {
    return "";
  }
  const tryExtract = (obj: unknown): string => {
    if (!obj || typeof obj !== "object") {
      return "";
    }
    const record = obj as Record<string, unknown>;
    const cmd = record.cmd;
    if (typeof cmd !== "string") {
      return "";
    }
    const trimmed = cmd.trim();
    if (!trimmed) {
      return "";
    }
    if (looksLikeJSONText(trimmed)) {
      try {
        const parsed = JSON.parse(trimmed) as unknown;
        const nested = tryExtract(parsed);
        if (nested) {
          return nested;
        }
      } catch {
        // ignore
      }
    }
    return trimmed;
  };

  const direct = tryExtract(value);
  if (direct) {
    return direct;
  }

  if (typeof value === "string") {
    const trimmed = value.trim();
    if (!trimmed) {
      return "";
    }
    try {
      const parsed = JSON.parse(trimmed) as unknown;
      return tryExtract(parsed);
    } catch {
      return "";
    }
  }

  return "";
}

function looksLikeJSONText(value: string): boolean {
  const trimmed = value.trim();
  return trimmed.startsWith("{") || trimmed.startsWith("[");
}

function normalizeToolOutput(value: string): string {
  const trimmed = value.trim();
  if (!trimmed || !looksLikeJSONText(trimmed)) {
    return value;
  }
  try {
    const parsed = JSON.parse(trimmed) as unknown;
    if (parsed && typeof parsed === "object" && "output" in parsed) {
      const output = (parsed as { output?: unknown }).output;
      if (typeof output === "string" && output.trim()) {
        return output.trim();
      }
    }
  } catch {
    // ignore
  }
  return value;
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
  const toolNames = new Map<string, string>();

  const reader = readline.createInterface({
    input: createReadStream(record.path, { encoding: "utf8" }),
    crlfDelay: Number.POSITIVE_INFINITY,
  });

  const pushDeduped = (block: PreviewBlock) => {
    const last = document.blocks[document.blocks.length - 1];
    if (
      last &&
      last.kind === block.kind &&
      (last.title || "") === (block.title || "") &&
      (last.body || "").trim() === (block.body || "").trim()
    ) {
      return;
    }
    document.blocks.push(block);
  };

  try {
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
          console.error(
            `[preview] Skipping malformed line in ${record.path}: ${
              error instanceof Error ? error.message : String(error)
            }`,
          );
        }
        continue;
      }

      if (envelope.type === "event_msg" || envelope.type === "event-msg") {
        const block = eventBlock(envelope.payload);
        if (block) {
          pushDeduped(block);
        }
        continue;
      }

      if (
        envelope.type === "response_item" ||
        envelope.type === "response-item"
      ) {
        for (const block of responseBlocks(envelope.payload, toolNames)) {
          pushDeduped(block);
        }
      }
    }
  } finally {
    reader.close();
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

    let header = escapeMarkdown(block.title?.trim() || block.kind);
    if (block.kind === "tool_call") {
      header = `Tool: ${header}`;
    } else if (block.kind === "tool_output") {
      header = `Output: ${header}`;
    }
    const body = block.body?.trim();

    if (!body) {
      sections.push(`## ${header}`);
      continue;
    }

    if (block.kind === "tool_call") {
      let fence = looksLikeJSONText(body) ? "json" : "bash";
      if (body.includes("*** Begin Patch")) {
        fence = "diff";
      }
      sections.push(`## ${header}`, `\`\`\`${fence}`, body, "```");
      continue;
    }

    if (block.kind === "tool_output") {
      const fence = looksLikeJSONText(body) ? "json" : "text";
      sections.push(`## ${header}`, `\`\`\`${fence}`, body, "```");
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
      if (!event.message?.trim()) {
        return null;
      }
      if (isInjectedAgentsContext(event.message)) {
        return {
          kind: "event",
          title: "Context",
          body: truncate(event.message, INJECTED_AGENTS_CONTEXT_LIMIT),
        };
      }
      return { kind: "user", title: "User", body: event.message };
    case "agent_message":
      if (!event.message?.trim()) {
        return null;
      }
      if (isInjectedAgentsContext(event.message)) {
        return {
          kind: "event",
          title: "Context",
          body: truncate(event.message, INJECTED_AGENTS_CONTEXT_LIMIT),
        };
      }
      return { kind: "assistant", title: "Assistant", body: event.message };
    case "task_started":
      return null;
    case "turn/completed":
    case "turn_started":
      return { kind: "event", title: event.type };
    default:
      return null;
  }
}

function responseBlocks(
  payload: unknown,
  toolNames: Map<string, string>,
): PreviewBlock[] {
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
      if (item.role === "user") {
        if (isInjectedAgentsContext(body)) {
          return [
            {
              kind: "event",
              title: "Context",
              body: truncate(body, INJECTED_AGENTS_CONTEXT_LIMIT),
            },
          ];
        }
        return [{ kind: "user", title: "User", body }];
      }
      if (isInjectedAgentsContext(body)) {
        return [
          {
            kind: "event",
            title: "Context",
            body: truncate(body, INJECTED_AGENTS_CONTEXT_LIMIT),
          },
        ];
      }
      return [{ kind: "assistant", title: "Assistant", body }];
    }
    case "function_call":
    case "tool_call":
    case "custom_tool_call": {
      const name = (item.name || "Tool Call").trim() || "Tool Call";
      const callId =
        typeof item.call_id === "string" ? item.call_id.trim() : "";
      if (callId) {
        toolNames.set(callId, name);
      }
      const cmd = cmdFromToolArguments(item.arguments);
      const input =
        typeof item.input === "string" && item.input.trim()
          ? item.input.trim()
          : "";
      const body =
        input || cmd || shortenJSON(item.input ?? item.arguments, 200);
      return [
        {
          kind: "tool_call",
          title: callId ? `${name} (${callId})` : name,
          body,
        },
      ];
    }
    case "function_call_output":
    case "tool_call_output":
    case "tool_output":
    case "custom_tool_call_output": {
      const callId =
        typeof item.call_id === "string" ? item.call_id.trim() : "";
      const name = callId ? toolNames.get(callId) : undefined;
      const output = normalizeToolOutput(
        item.output?.trim() || "command completed",
      );
      return [
        {
          kind: "tool_output",
          title:
            callId && name ? `${name} (${callId})` : callId || "Tool Output",
          body: truncate(output, 500),
        },
      ];
    }
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

function isInjectedAgentsContext(message: string): boolean {
  const trimmed = message.trim();
  if (!trimmed) {
    return false;
  }

  const firstLine = (trimmed.split("\n", 1)[0] || "").trim();
  const lower = trimmed.toLowerCase();
  const lowerFirst = firstLine.toLowerCase();

  const mentionsAgents =
    lowerFirst === "agents.md" ||
    lowerFirst.startsWith("# agents.md") ||
    lowerFirst.includes("agents.md instructions for");
  if (!mentionsAgents) {
    return false;
  }

  return (
    lower.includes("<instructions>") || lower.includes("<environment_context>")
  );
}
