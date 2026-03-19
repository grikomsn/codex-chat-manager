import { useEffect, useMemo } from "react";

import {
  SESSION_QUEUE_KEY,
  addToQueue,
  reconcileQueue,
  removeFromQueue,
} from "../queue";
import { useLocalStorageState } from "./use-session-state";

export function useQueueState(knownIDs: string[]): {
  queuedIDs: string[];
  queuedSet: Set<string>;
  toggleQueue: (id: string) => Promise<void>;
  clearQueue: () => Promise<void>;
  setQueuedIDs: (ids: string[]) => Promise<void>;
  removeFromQueue: (id: string) => Promise<void>;
  isLoading: boolean;
} {
  const {
    value: queuedIDs,
    setValue: setQueuedIDs,
    isLoading,
  } = useLocalStorageState<string[]>(SESSION_QUEUE_KEY, []);

  const queuedSet = useMemo(() => new Set(queuedIDs), [queuedIDs]);

  useEffect(() => {
    if (isLoading || knownIDs.length === 0) {
      return;
    }
    const next = reconcileQueue(queuedIDs, knownIDs);
    if (next.length !== queuedIDs.length) {
      void setQueuedIDs(next);
    }
  }, [knownIDs, isLoading, queuedIDs, setQueuedIDs]);

  const toggleQueue = async (id: string) => {
    const next = queuedSet.has(id)
      ? removeFromQueue(queuedIDs, id)
      : addToQueue(queuedIDs, id);
    await setQueuedIDs(next);
  };

  const clearQueue = async () => {
    await setQueuedIDs([]);
  };

  const setQueue = async (ids: string[]) => {
    await setQueuedIDs(ids);
  };

  const remove = async (id: string) => {
    await setQueuedIDs(removeFromQueue(queuedIDs, id));
  };

  return {
    queuedIDs,
    queuedSet,
    toggleQueue,
    clearQueue,
    setQueuedIDs: setQueue,
    removeFromQueue: remove,
    isLoading,
  };
}
