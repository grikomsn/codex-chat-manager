import type { SessionRecord } from "./types";

export function displayTitle(record: SessionRecord): string {
  if (record.title) {
    return record.title;
  }
  if (record.cwd) {
    const parts = record.cwd.split("/").filter(Boolean);
    return parts.at(-1) ?? record.cwd;
  }
  return record.id;
}

export function subtitle(record: SessionRecord): string {
  const cwd = record.cwd?.trim();
  const parts = cwd ? cwd.split("/").filter(Boolean) : [];
  const base = cwd ? (parts.at(-1) ?? cwd) : "unknown cwd";
  const project = record.project?.trim();
  const label =
    project && project !== base ? `${project} · ${base}` : project || base;
  if (record.agent_nickname) {
    return `${label} (${record.agent_nickname})`;
  }
  if (record.agent_role) {
    return `${label} (${record.agent_role})`;
  }
  return label;
}
