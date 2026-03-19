import { Cache } from "@raycast/api";

import type { SessionGroup } from "./types";

const cache = new Cache();

function cacheKey(status: string): string {
  return `session-groups:${status}`;
}

export function getCachedGroups(status: string): SessionGroup[] {
  const cached = cache.get(cacheKey(status));
  if (!cached) {
    return [];
  }
  try {
    return JSON.parse(cached) as SessionGroup[];
  } catch {
    return [];
  }
}

export function setCachedGroups(status: string, groups: SessionGroup[]): void {
  cache.set(cacheKey(status), JSON.stringify(groups));
}
