import {
  Action,
  ActionPanel,
  Alert,
  Color,
  Icon,
  List,
  Toast,
  confirmAlert,
  openExtensionPreferences,
  showToast,
  useNavigation,
} from "@raycast/api";
import { useCachedPromise } from "@raycast/utils";
import { useCallback, useEffect, useMemo, useState } from "react";

import { getCachedGroups, setCachedGroups } from "../cache";
import { getManagerRuntime, runAction, useSessionGroups } from "../cli";
import {
  canDeleteGroups,
  childGroups,
  countRenderableBlocks,
  formatDate,
  groupKeywords,
  shouldAllowArchive,
  shouldAllowUnarchive,
  statusIcon,
  statusLabel,
} from "../format";
import { useLocalStorageState, useQueueState } from "../hooks";
import { parsePreviewFromFile, renderPreviewMarkdown } from "../lib";
import { SHOW_SYSTEM_KEY } from "../queue";
import { displayTitle, subtitle } from "../record";
import type {
  MutationAction,
  PreviewDocument,
  SessionGroup,
  SessionRecord,
  SessionStatusFilter,
} from "../types";

interface SessionBrowserProps {
  title: string;
  statusFilter: SessionStatusFilter;
  groups?: SessionGroup[];
  onRevalidateParent?: () => Promise<void> | void;
}

export default function SessionBrowser(props: SessionBrowserProps) {
  const runtime = useMemo(() => getManagerRuntime(), []);
  const { pop } = useNavigation();
  const initialGroups = props.groups;
  const cachedGroups = useMemo(
    () => getCachedGroups(props.statusFilter),
    [props.statusFilter],
  );
  const [searchText, setSearchText] = useState("");
  const [selectedID, setSelectedID] = useState<string | undefined>(
    initialGroups?.[0]?.parent.id ?? cachedGroups[0]?.parent.id,
  );
  const { value: showSystem, setValue: setShowSystem } =
    useLocalStorageState<boolean>(SHOW_SYSTEM_KEY, false);

  const topLevel = !initialGroups;

  const sessionState = useSessionGroups(
    runtime,
    props.statusFilter,
    cachedGroups,
    topLevel,
  );
  const groups = initialGroups ?? sessionState?.data ?? cachedGroups;
  const knownTopLevelIDs = useMemo(
    () => groups.map((group) => group.parent.id),
    [groups],
  );

  useEffect(() => {
    if (topLevel && sessionState?.data) {
      setCachedGroups(props.statusFilter, sessionState.data);
    }
  }, [topLevel, props.statusFilter, sessionState?.data]);

  useEffect(() => {
    if (!selectedID || groups.some((group) => group.parent.id === selectedID)) {
      return;
    }
    setSelectedID(groups[0]?.parent.id);
  }, [groups, selectedID]);

  const selectedGroup = useMemo(
    () => groups.find((group) => group.parent.id === selectedID) ?? groups[0],
    [groups, selectedID],
  );
  const selectedRecord = selectedGroup?.parent;
  const previewState = useCachedPromise(
    async (record: SessionRecord | undefined) =>
      record ? parsePreviewFromFile(record) : null,
    [selectedRecord],
    {
      execute: Boolean(selectedRecord),
      keepPreviousData: true,
    },
  );

  const selectedMarkdown = useMemo(() => {
    if (!selectedGroup) {
      return "# No Conversation Selected";
    }
    if (previewState.error) {
      return `# ${displayTitle(selectedGroup.parent)}\n\nFailed to load preview.\n\n${previewState.error.message}`;
    }
    if (!previewState.data) {
      return `# ${displayTitle(selectedGroup.parent)}\n\nLoading preview...`;
    }
    return renderPreviewMarkdown(previewState.data, showSystem);
  }, [previewState.data, previewState.error, selectedGroup, showSystem]);

  const {
    queuedIDs,
    queuedSet,
    toggleQueue,
    clearQueue,
    setQueuedIDs,
    isLoading: queueLoading,
  } = useQueueState(topLevel ? knownTopLevelIDs : []);

  const revalidateAll = useCallback(async () => {
    if (topLevel) {
      await sessionState?.revalidate?.();
      return;
    }
    await props.onRevalidateParent?.();
  }, [topLevel, sessionState, props.onRevalidateParent]);

  const queuedGroups = useMemo(
    () =>
      topLevel ? groups.filter((group) => queuedSet.has(group.parent.id)) : [],
    [groups, queuedSet, topLevel],
  );

  const runMutation = async (
    action: MutationAction,
    targetGroups: SessionGroup[],
    options?: { bulk?: boolean },
  ) => {
    if (targetGroups.length === 0) {
      return;
    }

    const ids = targetGroups.map((group) => group.parent.id);
    const destructive = action === "delete";
    const confirmed = await confirmAlert({
      title: `${titleCase(action)} ${ids.length} Conversation${ids.length === 1 ? "" : "s"}?`,
      message: destructive
        ? "Delete only removes archived rollout files and matching sidecars."
        : `This will ${action} the selected conversation${ids.length === 1 ? "" : "s"}.`,
      primaryAction: {
        title: titleCase(action),
        style: destructive
          ? Alert.ActionStyle.Destructive
          : Alert.ActionStyle.Default,
      },
    });
    if (!confirmed) {
      return;
    }

    const toast = await showToast({
      style: Toast.Style.Animated,
      title: `${titleCase(action)}ing Conversation${ids.length === 1 ? "" : "s"}...`,
    });

    try {
      const plan = await runAction(runtime, action, ids);
      toast.style = Toast.Style.Success;
      toast.title = `${titleCase(action)} Complete`;
      toast.message = summarizeActionPlan(plan);

      if (topLevel && options?.bulk) {
        await setQueuedIDs(
          queuedIDs.filter((id) => !plan.target_ids.includes(id)),
        );
      }
      if (topLevel && !options?.bulk) {
        await setQueuedIDs(queuedIDs.filter((id) => id !== ids[0]));
      }

      await revalidateAll();
      if (!topLevel) {
        pop();
      }
    } catch (error) {
      toast.style = Toast.Style.Failure;
      toast.title = `Failed to ${action}`;
      toast.message = error instanceof Error ? error.message : String(error);
    }
  };

  const bulkArchiveGroups = queuedGroups.filter(shouldAllowArchive);
  const bulkUnarchiveGroups = queuedGroups.filter(shouldAllowUnarchive);
  const canBulkDelete = canDeleteGroups(queuedGroups);

  return (
    <List
      isLoading={Boolean(topLevel && sessionState?.isLoading) || queueLoading}
      isShowingDetail
      searchText={searchText}
      onSearchTextChange={setSearchText}
      searchBarPlaceholder={`Search ${props.title.toLowerCase()}`}
      navigationTitle={props.title}
      onSelectionChange={(id) => {
        setSelectedID(id ?? undefined);
      }}
      filtering
      throttle
    >
      <List.EmptyView
        title={topLevel ? "No Conversations" : "No Child Conversations"}
        description={
          topLevel
            ? "No matching Codex conversations were found. Check your binary path or CODEX_HOME preferences."
            : "This conversation has no child sessions."
        }
        actions={
          <ActionPanel>
            <Action
              title="Open Extension Preferences"
              onAction={openExtensionPreferences}
            />
            <Action title="Refresh" onAction={revalidateAll} />
          </ActionPanel>
        }
      />
      {groups.map((group) => {
        const isSelected = selectedGroup?.parent.id === group.parent.id;
        const detailDoc: PreviewDocument | null = isSelected
          ? (previewState.data ?? null)
          : null;
        const markdown = isSelected
          ? selectedMarkdown
          : `# ${displayTitle(group.parent)}\n\nSelect this conversation to load its transcript preview.`;
        const renderableCount = detailDoc
          ? countRenderableBlocks(detailDoc.blocks, showSystem)
          : undefined;
        const icon = statusIcon(group.status, group.mixed_status);
        const queued = queuedSet.has(group.parent.id);

        return (
          <List.Item
            key={group.parent.id}
            id={group.parent.id}
            icon={icon}
            title={displayTitle(group.parent)}
            subtitle={subtitle(group.parent)}
            keywords={groupKeywords(group)}
            accessories={[
              { text: formatDate(group.aggregate_at) },
              group.child_count > 0
                ? { text: `${group.child_count} children` }
                : { text: "solo" },
              ...(queued ? [{ icon: Icon.Checkmark, tooltip: "Queued" }] : []),
              {
                tag: {
                  value: statusLabel(group.status, group.mixed_status),
                  color:
                    group.status === "archived"
                      ? Color.SecondaryText
                      : group.status === "mixed"
                        ? Color.Yellow
                        : Color.Green,
                },
              },
            ]}
            detail={
              <List.Item.Detail
                markdown={markdown}
                metadata={
                  <List.Item.Detail.Metadata>
                    <List.Item.Detail.Metadata.Label
                      title="Status"
                      text={statusLabel(group.status, group.mixed_status)}
                    />
                    <List.Item.Detail.Metadata.Label
                      title="Updated"
                      text={formatDate(group.aggregate_at)}
                    />
                    <List.Item.Detail.Metadata.Label
                      title="Session ID"
                      text={group.parent.id}
                    />
                    <List.Item.Detail.Metadata.Label
                      title="CWD"
                      text={group.parent.cwd || "unknown cwd"}
                    />
                    <List.Item.Detail.Metadata.Label
                      title="Source"
                      text={group.parent.source || "unknown"}
                    />
                    <List.Item.Detail.Metadata.Label
                      title="Children"
                      text={String(group.child_count)}
                    />
                    <List.Item.Detail.Metadata.Label
                      title="Path"
                      text={group.parent.path}
                    />
                    {renderableCount !== undefined ? (
                      <List.Item.Detail.Metadata.Label
                        title="Visible Blocks"
                        text={String(renderableCount)}
                      />
                    ) : null}
                  </List.Item.Detail.Metadata>
                }
              />
            }
            actions={
              <ActionPanel>
                {group.children?.length ? (
                  <Action.Push
                    title="Open Child Conversations"
                    icon={Icon.Sidebar}
                    target={
                      <SessionBrowser
                        title={`${displayTitle(group.parent)} Children`}
                        statusFilter="all"
                        groups={childGroups(group)}
                        onRevalidateParent={revalidateAll}
                      />
                    }
                  />
                ) : null}
                <Action
                  title={queued ? "Remove from Queue" : "Add to Queue"}
                  icon={queued ? Icon.MinusCircle : Icon.PlusCircle}
                  onAction={() => void toggleQueue(group.parent.id)}
                />
                {shouldAllowArchive(group) ? (
                  <Action
                    title="Archive Conversation"
                    onAction={() => void runMutation("archive", [group])}
                  />
                ) : null}
                {shouldAllowUnarchive(group) ? (
                  <Action
                    title="Unarchive Conversation"
                    icon={Icon.Box}
                    onAction={() => void runMutation("unarchive", [group])}
                  />
                ) : null}
                {group.status === "archived" && !group.mixed_status ? (
                  <Action
                    title="Delete Conversation"
                    style={Action.Style.Destructive}
                    icon={Icon.Trash}
                    onAction={() => void runMutation("delete", [group])}
                  />
                ) : null}
                <Action
                  title={
                    showSystem ? "Hide Context Blocks" : "Show Context Blocks"
                  }
                  icon={showSystem ? Icon.EyeDisabled : Icon.Eye}
                  onAction={() => {
                    void setShowSystem(!showSystem);
                  }}
                />
                {topLevel && queuedGroups.length > 0 ? (
                  <ActionPanel.Section title={`Queue (${queuedGroups.length})`}>
                    {bulkArchiveGroups.length > 0 ? (
                      <Action
                        title={`Archive Queued (${bulkArchiveGroups.length})`}
                        onAction={() =>
                          void runMutation("archive", bulkArchiveGroups, {
                            bulk: true,
                          })
                        }
                      />
                    ) : null}
                    {bulkUnarchiveGroups.length > 0 ? (
                      <Action
                        title={`Unarchive Queued (${bulkUnarchiveGroups.length})`}
                        icon={Icon.Box}
                        onAction={() =>
                          void runMutation("unarchive", bulkUnarchiveGroups, {
                            bulk: true,
                          })
                        }
                      />
                    ) : null}
                    {canBulkDelete ? (
                      <Action
                        title={`Delete Queued (${queuedGroups.length})`}
                        icon={Icon.Trash}
                        style={Action.Style.Destructive}
                        onAction={() =>
                          void runMutation("delete", queuedGroups, {
                            bulk: true,
                          })
                        }
                      />
                    ) : null}
                    <Action
                      title="Clear Queue"
                      icon={Icon.XMarkCircle}
                      onAction={() => void clearQueue()}
                    />
                  </ActionPanel.Section>
                ) : null}
                <ActionPanel.Section>
                  <Action
                    title="Refresh"
                    icon={Icon.RotateClockwise}
                    onAction={() => void revalidateAll()}
                  />
                  <Action
                    title="Open Extension Preferences"
                    icon={Icon.Gear}
                    onAction={openExtensionPreferences}
                  />
                </ActionPanel.Section>
              </ActionPanel>
            }
          />
        );
      })}
    </List>
  );
}

function summarizeActionPlan(plan: {
  target_ids: string[];
  skipped?: { reason: string }[];
  blocked_by_active_ids?: string[];
}): string {
  const changed = `${plan.target_ids.length} changed`;
  const skipped = plan.skipped?.length
    ? `, ${plan.skipped.length} skipped`
    : "";
  const blocked = plan.blocked_by_active_ids?.length
    ? `, blocked by ${plan.blocked_by_active_ids.length} active`
    : "";
  return `${changed}${skipped}${blocked}`;
}

function titleCase(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
