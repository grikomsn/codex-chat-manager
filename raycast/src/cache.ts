import { Cache } from "@raycast/api";

import type { SessionGroup } from "./types";

const cache = new Cache();

const DEFAULT_TTL = 300000;

interface CachedGroupsEntry {
  data: SessionGroup[];
  timestamp: number;
}

function cacheKey(status: string): string {
  return `session-groups:${status}`;
}

export function getCachedGroups(status: string): SessionGroup[] {
  const cached = cache.get(cacheKey(status));
  if (!cached) {
    return [];
  }

  let entry: CachedGroupsEntry;
  try {
    entry = JSON.parse(cached) as CachedGroupsEntry;
  } catch {
    console.warn("Malformed cache entry, clearing");
    cache.remove(cacheKey(status));
    return [];
  }

  if (Date.now() - entry.timestamp > DEFAULT_TTL) {
    cache.remove(cacheKey(status));
    return [];
  }

  return entry.data;
}

export function setCachedGroups(status: string, groups: SessionGroup[]): void {
  const entry: CachedGroupsEntry = {
    data: groups,
    timestamp: Date.now(),
  };
  cache.set(cacheKey(status), JSON.stringify(entry));
}

export function clearCachedGroups(status: string): void {
  cache.remove(cacheKey(status));
}
