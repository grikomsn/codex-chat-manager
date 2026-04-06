import type { MutationAction, SessionStatusFilter } from "./types";

export interface ManagerErrorPayload {
  code?: string;
  message: string;
  details?: unknown;
}

export interface ManagerResponseEnvelope<TData = unknown> {
  schema_version: string;
  command: string;
  ok: boolean;
  data?: TData;
  error?: ManagerErrorPayload;
}

export interface ParsedManagerResponse<TData> {
  data: TData;
  envelope?: ManagerResponseEnvelope<TData>;
}

const managerSchemaVersion = "1";

interface ManagerEnvelopeShape {
  schema_version: string;
  command: string;
  ok: boolean;
  error?: unknown;
}

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

export class ManagerResponseError extends Error {
  readonly response: ManagerResponseEnvelope<unknown>;
  readonly schemaVersion: string;
  readonly command: string;
  readonly remoteError?: ManagerErrorPayload;
  readonly data?: unknown;
  readonly code?: string;
  readonly details?: unknown;
  readonly blockedIds?: string[];

  constructor(response: ManagerResponseEnvelope<unknown>) {
    super(formatManagerResponseErrorMessage(response));
    this.name = "ManagerResponseError";
    this.response = response;
    this.schemaVersion = response.schema_version;
    this.command = response.command;
    this.remoteError = response.error;
    this.data = response.data;
    this.code = response.error?.code;
    this.details = response.error?.details;
    this.blockedIds =
      extractBlockedIDs(response.data) ??
      extractBlockedIDs(response.error?.details);
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isManagerEnvelopeShape(value: unknown): value is ManagerEnvelopeShape {
  if (!isRecord(value)) {
    return false;
  }
  return (
    typeof value.schema_version === "string" &&
    typeof value.command === "string" &&
    typeof value.ok === "boolean"
  );
}

function isValidManagerError(value: unknown, ok: boolean): boolean {
  if (ok) {
    return value === undefined || isRecord(value);
  }
  return isRecord(value) && typeof value.message === "string";
}

function isManagerEnvelope(
  value: unknown,
): value is ManagerResponseEnvelope<unknown> {
  if (!isManagerEnvelopeShape(value)) {
    return false;
  }
  return isValidManagerError(value.error, value.ok);
}

function managerSchemaVersionMismatchMessage(version: unknown): string {
  return `Unsupported codex-chat-manager schema version: ${String(version)}`;
}

function extractBlockedIDs(value: unknown): string[] | undefined {
  if (!isRecord(value)) {
    return undefined;
  }
  const raw = value.blocked_by_active_ids;
  if (!Array.isArray(raw)) {
    return undefined;
  }
  const blockedIds = raw.filter(
    (item): item is string => typeof item === "string" && item.length > 0,
  );
  return blockedIds.length > 0 ? blockedIds : undefined;
}

export function formatManagerResponseErrorMessage(
  response: ManagerResponseEnvelope<unknown>,
): string {
  const error = response.error;
  const blockedIds =
    extractBlockedIDs(response.data) ?? extractBlockedIDs(error?.details);
  const parts: string[] = [];
  if (error?.code) {
    parts.push(`[${error.code}]`);
  }
  parts.push(error?.message ?? `codex-chat-manager ${response.command} failed`);
  if (blockedIds?.length) {
    parts.push(`blocked IDs: ${blockedIds.join(", ")}`);
  }
  return parts.join(" ");
}

export function parseManagerResponse<T>(
  stdout: string,
): ParsedManagerResponse<T> {
  const trimmed = stdout.trim();
  if (!trimmed) {
    throw new Error("Empty response from codex-chat-manager");
  }

  const clean = trimmed.charCodeAt(0) === 0xfeff ? trimmed.slice(1) : trimmed;

  try {
    const parsed: unknown = JSON.parse(clean);
    if (!isManagerEnvelopeShape(parsed)) {
      throw new ParserError(
        "Expected codex-chat-manager response envelope",
        clean.slice(0, 200),
      );
    }
    if (parsed.schema_version !== managerSchemaVersion) {
      throw new ParserError(
        managerSchemaVersionMismatchMessage(parsed.schema_version),
        clean.slice(0, 200),
      );
    }
    if (!isManagerEnvelope(parsed)) {
      throw new ParserError(
        "Malformed codex-chat-manager response envelope",
        clean.slice(0, 200),
      );
    }
    const envelope = parsed as ManagerResponseEnvelope<T>;
    return {
      data: (envelope.data as T) ?? (undefined as T),
      envelope,
    };
  } catch (error) {
    if (error instanceof ParserError) {
      throw error;
    }
    const snippet = clean.slice(0, 200);
    throw new ParserError(
      `Failed to parse codex-chat-manager JSON output: ${error instanceof Error ? error.message : String(error)}`,
      snippet,
    );
  }
}

export function parseManagerJSON<T>(stdout: string): T {
  const response = parseManagerResponse<T>(stdout);
  if (response.envelope && !response.envelope.ok) {
    throw new ManagerResponseError(response.envelope);
  }
  return response.data;
}

export function extractManagerResponseError(
  stdout: string,
): ManagerResponseError | undefined {
  try {
    const response = parseManagerResponse<unknown>(stdout);
    if (response.envelope && !response.envelope.ok) {
      return new ManagerResponseError(response.envelope);
    }
  } catch {
    return undefined;
  }
  return undefined;
}
