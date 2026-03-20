import { Color, Icon } from "@raycast/api";

import { displayTitle } from "./record";
import type { PreviewBlock, SessionGroup, Status } from "./types";

export function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

export function statusLabel(status: Status): string {
  return status;
}

export function statusIcon(status: Status): { source: Icon; tintColor: Color } {
  if (status === "mixed") {
    return { source: Icon.CircleFilled, tintColor: Color.Yellow };
  }
  if (status === "archived") {
    return { source: Icon.Circle, tintColor: Color.SecondaryText };
  }
  return { source: Icon.CircleFilled, tintColor: Color.Green };
}

export interface StatusMeta {
  label: string;
  icon: { source: Icon; tintColor: Color };
}

export function getStatusMeta(status: Status): StatusMeta {
  const label = statusLabel(status);
  const icon = statusIcon(status);
  return { label, icon };
}

export function groupKeywords(group: SessionGroup): string[] {
  const values = [
    group.parent.id,
    displayTitle(group.parent),
    group.parent.cwd,
    group.parent.project,
    group.parent.source,
    group.parent.agent_nickname,
    group.parent.agent_role,
    ...(group.children ?? []).flatMap((child) => [
      child.id,
      displayTitle(child),
      child.cwd,
      child.project,
      child.agent_nickname,
      child.agent_role,
    ]),
  ];
  return values.filter((value): value is string => Boolean(value));
}

export function childGroups(group: SessionGroup): SessionGroup[] {
  return (group.children ?? []).map((child) => ({
    parent: child,
    children: [],
    status: child.status,
    aggregate_at: child.updated_at,
    child_count: 0,
    cascades_to: [child.id],
    parent_exists: true,
  }));
}

export function canDeleteGroups(groups: SessionGroup[]): boolean {
  return groups.length > 0;
}

export function shouldAllowArchive(group: SessionGroup): boolean {
  return group.status !== "archived";
}

export function shouldAllowUnarchive(group: SessionGroup): boolean {
  return group.status !== "active";
}

export function countRenderableBlocks(
  blocks: PreviewBlock[],
  showSystem: boolean,
): number {
  let skippedFirstAssistant = false;
  const hasUser = blocks.some((block) => block.kind === "user");
  return blocks.filter((block) => {
    if (!showSystem && block.title === "Context") {
      return false;
    }
    if (!showSystem && hasUser && !skippedFirstAssistant && block.kind === "assistant") {
      skippedFirstAssistant = true;
      return false;
    }
    if (block.kind === "assistant" && !skippedFirstAssistant) {
      skippedFirstAssistant = true;
    }
    return true;
  }).length;
}
