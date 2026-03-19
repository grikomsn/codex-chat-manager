export type Status = "active" | "archived" | "mixed";

export type SessionStatusFilter = "all" | "active" | "archived";

export interface SessionRecord {
  id: string;
  path: string;
  status: Status;
  created_at: string;
  updated_at: string;
  cwd?: string;
  title?: string;
  source?: string;
  agent_nickname?: string;
  agent_role?: string;
  parent_id?: string;
  child_count: number;
  size_bytes: number;
  is_orphan?: boolean;
  has_preview: boolean;
}

export interface SessionGroup {
  parent: SessionRecord;
  children?: SessionRecord[];
  status: Status;
  aggregate_at: string;
  mixed_status: boolean;
  child_count: number;
  cascades_to: string[];
  parent_exists: boolean;
}

export type PreviewBlockKind =
  | "user"
  | "assistant"
  | "tool_call"
  | "tool_output"
  | "event";

export interface PreviewBlock {
  kind: PreviewBlockKind;
  title?: string;
  body?: string;
}

export interface PreviewDocument {
  sessionId: string;
  title: string;
  blocks: PreviewBlock[];
}

export type MutationAction = "archive" | "unarchive" | "delete";

export interface ActionTarget {
  id: string;
  path: string;
  status: Status;
  parent_id?: string;
  is_child: boolean;
  is_selected?: boolean;
}

export interface ActionSkip {
  id?: string;
  path?: string;
  reason: string;
}

export interface ActionPlan {
  type: MutationAction;
  requested_ids: string[];
  target_ids: string[];
  targets: ActionTarget[];
  skipped?: ActionSkip[];
  removed_index_rows?: number;
  removed_snapshots?: string[];
  blocked_by_active_ids?: string[];
}
