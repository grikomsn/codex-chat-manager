import { execFile } from "node:child_process";
import { promisify } from "node:util";

import { useExec } from "@raycast/utils";

import {
  actionArgs,
  formatManagerResponseErrorMessage,
  listArgs,
  ManagerResponseError,
  parseManagerResponse,
  ParserError,
} from "./cli-core";
import { getManagerPreferences, trimPreference } from "./preferences";
import type {
  ActionPlan,
  MutationAction,
  SessionGroup,
  SessionStatusFilter,
} from "./types";

const execFileAsync = promisify(execFile);

export interface ManagerRuntime {
  binaryPath: string;
  env: NodeJS.ProcessEnv;
}

export class ManagerCommandError extends Error {
  readonly command: string;
  readonly stdout?: string;
  readonly stderr?: string;
  readonly exitCode?: number;
  readonly code?: string;
  readonly blockedIds?: string[];
  readonly details?: unknown;
  readonly partialData?: unknown;

  constructor(
    message: string,
    command: string,
    options?: {
      stdout?: string;
      stderr?: string;
      exitCode?: number | null;
      code?: string;
      blockedIds?: string[];
      details?: unknown;
      partialData?: unknown;
    },
  ) {
    super(message);
    this.name = "ManagerCommandError";
    this.command = command;
    this.stdout = options?.stdout;
    this.stderr = options?.stderr;
    this.exitCode = options?.exitCode ?? undefined;
    this.code = options?.code;
    this.blockedIds = options?.blockedIds;
    this.details = options?.details;
    this.partialData = options?.partialData;
  }
}

function hasStructuredFailure(
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

function buildManagerCommandError(
  failure: ManagerResponseError,
  command: string,
  options?: {
    stdout?: string;
    stderr?: string;
    exitCode?: number | null;
  },
): ManagerCommandError {
  return new ManagerCommandError(
    formatManagerResponseErrorMessage(failure.response),
    command,
    {
      ...options,
      code: failure.code,
      blockedIds: failure.blockedIds,
      details: failure.details,
      partialData: failure.data,
    },
  );
}

export function managerCommandErrorFromOutput(
  stdout: string,
  command: string,
  options?: {
    stderr?: string;
    exitCode?: number | null;
  },
): ManagerCommandError | undefined {
  const failure = hasStructuredFailure(stdout);
  if (!failure) {
    return undefined;
  }
  return buildManagerCommandError(failure, command, {
    stdout,
    stderr: options?.stderr,
    exitCode: options?.exitCode,
  });
}

export function getManagerRuntime(): ManagerRuntime {
  const preferences = getManagerPreferences();
  const binaryPath =
    trimPreference(preferences.managerBinaryPath) ?? "codex-chat-manager";
  const codexHome = trimPreference(preferences.codexHome);

  return {
    binaryPath,
    env: codexHome ? { ...process.env, CODEX_HOME: codexHome } : process.env,
  };
}

export function useSessionGroups(
  runtime: ManagerRuntime,
  status: SessionStatusFilter,
  initialData: SessionGroup[],
  execute = true,
) {
  return useExec<SessionGroup[], SessionGroup[]>(
    runtime.binaryPath,
    listArgs(status),
    {
      encoding: "utf8",
      env: runtime.env,
      execute,
      initialData,
      keepPreviousData: true,
      parseOutput: ({ stdout, stderr, exitCode, command }) => {
        const commandError = managerCommandErrorFromOutput(stdout, command, {
          stderr,
          exitCode: exitCode ?? undefined,
        });
        if (commandError) {
          throw commandError;
        }
        if (exitCode && exitCode !== 0) {
          throw new ManagerCommandError(
            stderr || `codex-chat-manager exited with code ${exitCode}`,
            command,
            { stdout, stderr, exitCode: exitCode ?? undefined },
          );
        }
        const response = parseManagerResponse<SessionGroup[]>(stdout);
        return response.data;
      },
    },
  );
}

export async function runAction(
  runtime: ManagerRuntime,
  action: MutationAction,
  ids: string[],
): Promise<ActionPlan> {
  const args = actionArgs(action, ids);
  try {
    const { stdout } = await execFileAsync(runtime.binaryPath, args, {
      env: runtime.env,
    });
    const commandError = managerCommandErrorFromOutput(
      stdout,
      [runtime.binaryPath, ...args].join(" "),
    );
    if (commandError) {
      throw commandError;
    }
    const response = parseManagerResponse<ActionPlan>(stdout);
    return response.data;
  } catch (error) {
    if (error instanceof ParserError || error instanceof ManagerResponseError) {
      throw error;
    }
    const stdout =
      error instanceof Error &&
      "stdout" in error &&
      typeof error.stdout === "string"
        ? error.stdout
        : "";
    const commandError = stdout
      ? managerCommandErrorFromOutput(
          stdout,
          [runtime.binaryPath, ...args].join(" "),
          {
            stderr:
              error instanceof Error &&
              "stderr" in error &&
              typeof error.stderr === "string"
                ? error.stderr
                : undefined,
            exitCode:
              error instanceof Error &&
              "exitCode" in error &&
              typeof error.exitCode === "number"
                ? error.exitCode
                : undefined,
          },
        )
      : undefined;
    if (commandError) {
      throw commandError;
    }
    const message =
      error instanceof Error &&
      "stderr" in error &&
      typeof error.stderr === "string" &&
      error.stderr.trim()
        ? error.stderr.trim()
        : error instanceof Error
          ? error.message
          : String(error);
    throw new ManagerCommandError(
      message,
      [runtime.binaryPath, ...args].join(" "),
      {
        stdout,
        stderr:
          error instanceof Error &&
          "stderr" in error &&
          typeof error.stderr === "string"
            ? error.stderr
            : undefined,
        exitCode:
          error instanceof Error &&
          "exitCode" in error &&
          typeof error.exitCode === "number"
            ? error.exitCode
            : undefined,
      },
    );
  }
}
