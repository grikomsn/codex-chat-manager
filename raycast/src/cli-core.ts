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

export function parseManagerJSON<T>(stdout: string): T {
  try {
    return JSON.parse(stdout) as T;
  } catch (error) {
    throw new Error(
      `Failed to parse codex-chat-manager JSON output: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}
