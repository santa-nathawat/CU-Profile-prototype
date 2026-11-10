'use client';

import { useSyncExternalStore } from 'react';

let current: string | null = null;
const listeners = new Set<() => void>();

function set(message: string | null) {
  if (message === current) return;
  current = message;
  listeners.forEach((listener) => listener());
}

/**
 * For a blocked action whose row may vanish when the page refreshes. The message
 * lives outside the row, so the student still reads why it didn't go through.
 */
export const showNotice = (message: string) => set(message);
export const dismissNotice = () => set(null);

export function useNotice(): string | null {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    () => current,
    () => null,
  );
}
