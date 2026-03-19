export const SESSION_QUEUE_KEY = "selected-session-ids";
export const SHOW_SYSTEM_KEY = "show-system-context";

export function addToQueue(ids: string[], id: string): string[] {
  return ids.includes(id) ? ids : [...ids, id];
}

export function removeFromQueue(ids: string[], id: string): string[] {
  return ids.filter((candidate) => candidate !== id);
}

export function toggleQueue(ids: string[], id: string): string[] {
  return ids.includes(id) ? removeFromQueue(ids, id) : addToQueue(ids, id);
}

export function reconcileQueue(ids: string[], knownIDs: string[]): string[] {
  const allowed = new Set(knownIDs);
  return ids.filter((id) => allowed.has(id));
}
