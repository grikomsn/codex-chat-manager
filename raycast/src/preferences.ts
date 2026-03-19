import { getPreferenceValues } from "@raycast/api";

export interface ExtensionPreferences {
  managerBinaryPath?: string;
  codexHome?: string;
}

export function getManagerPreferences(): ExtensionPreferences {
  return getPreferenceValues<ExtensionPreferences>();
}

export function trimPreference(value?: string): string | undefined {
  const trimmed = value?.trim();
  return trimmed ? trimmed : undefined;
}
