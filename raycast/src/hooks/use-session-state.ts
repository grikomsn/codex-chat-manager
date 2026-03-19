import { useLocalStorage } from "@raycast/utils";

export function useLocalStorageState<T>(
  key: string,
  defaultValue: T,
): {
  value: T;
  setValue: (value: T | ((prev: T) => T)) => Promise<void>;
  isLoading: boolean;
} {
  const {
    value: storedValue,
    setValue,
    isLoading,
  } = useLocalStorage<T>(key, defaultValue);
  const value = storedValue ?? defaultValue;
  return {
    value,
    setValue: setValue as (value: T | ((prev: T) => T)) => Promise<void>,
    isLoading,
  };
}
