'use client';

import { useSyncExternalStore } from 'react';

const STEP_MS = 10_000;

function subscribe(onTick: () => void) {
  const timer = setInterval(onTick, STEP_MS);
  return () => clearInterval(timer);
}

function currentStep() {
  return Math.floor(Date.now() / STEP_MS) * STEP_MS;
}

/**
 * The current time, re-rendering as it passes. Null during server rendering and
 * hydration, so anything built on it can't cause a hydration mismatch.
 */
export function useNow(): number | null {
  return useSyncExternalStore(subscribe, currentStep, () => null);
}
