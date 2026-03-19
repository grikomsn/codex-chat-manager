import { useExec } from "@raycast/utils";
import { execFile } from "node:child_process";
import { promisify } from "node:util";

import { actionArgs, listArgs, parseManagerJSON } from "./cli-core";
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
  command: string;

  constructor(message: string, command: string) {
    super(message);
    this.name = "ManagerCommandError";
    this.command = command;
  }
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
        if (exitCode && exitCode !== 0) {
          throw new ManagerCommandError(
            stderr || `codex-chat-manager exited with code ${exitCode}`,
            command,
          );
        }
        return parseManagerJSON<SessionGroup[]>(stdout);
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
    return parseManagerJSON<ActionPlan>(stdout);
  } catch (error) {
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
    );
  }
}
