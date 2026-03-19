import type { MutationAction, SessionStatusFilter } from "./types";

export function listArgs(status: SessionStatusFilter): string[] {
  const args = ["sessions", "list", "--json"];
  if (status !== "all") {
    args.push("--status", status);
  }
  return args;
}

export function actionArgs(action: MutationAction, ids: string[]): string[] {
  const args = ["sessions", action];
  for (const id of ids) {
    args.push("--id", id);
  }
  if (action === "delete") {
    args.push("--yes");
  }
  args.push("--json");
  return args;
}

export class ParserError extends Error {
  constructor(
    message: string,
    readonly snippet: string,
  ) {
    super(message);
    this.name = "ParserError";
  }
}

export function parseManagerJSON<T>(stdout: string): T {
  const trimmed = stdout.trim();
  if (!trimmed) {
    throw new Error("Empty response from codex-chat-manager");
  }

  const clean = trimmed.charCodeAt(0) === 0xfeff ? trimmed.slice(1) : trimmed;

  try {
    return JSON.parse(clean) as T;
  } catch (error) {
    const snippet = clean.slice(0, 200);
    throw new ParserError(
      `Failed to parse codex-chat-manager JSON output: ${error instanceof Error ? error.message : String(error)}`,
      snippet,
    );
  }
}
