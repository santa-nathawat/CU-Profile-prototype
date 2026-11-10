'use client';

import { useEffect } from 'react';
import { usePathname, useSearchParams } from 'next/navigation';

const CURRENT = 'cu:nav:current';
const PREVIOUS = 'cu:nav:previous';

function read(key: string): string | null {
  try {
    return sessionStorage.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string) {
  try {
    sessionStorage.setItem(key, value);
  } catch {
    // Storage can be blocked. BackLink then falls back to a plain link.
  }
}

/** The page shown before this one in this tab, with its query string. */
export function previousPath(): string | null {
  return read(PREVIOUS);
}

export function NavTracker() {
  const pathname = usePathname();
  const search = useSearchParams().toString();

  useEffect(() => {
    const url = search ? `${pathname}?${search}` : pathname;
    const current = read(CURRENT);
    if (current === url) return;
    if (current) write(PREVIOUS, current);
    write(CURRENT, url);
  }, [pathname, search]);

  return null;
}
