'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { backLabel } from '@/lib/nav';
import { ChevronLeftIcon } from './ui/icons';
import { previousPath } from './shell/NavTracker';

/**
 * Returns to the list a page was opened from. When that list is the previous
 * page in this tab, going back in history restores its scroll position. A
 * shared or reloaded link has no such history, so it links to the list URL.
 */
export function BackLink({
  from,
  fallback,
}: {
  from: string | null;
  fallback: string;
}) {
  const router = useRouter();
  const target = from ?? fallback;

  return (
    <Link
      href={target}
      onClick={(event) => {
        if (event.metaKey || event.ctrlKey || event.shiftKey) return;
        if (previousPath() === target) {
          event.preventDefault();
          router.back();
        }
      }}
      className="-ml-2 inline-flex min-h-9 items-center gap-1 rounded-lg px-2 text-sm font-medium text-ink-muted hover:bg-sunken hover:text-ink"
    >
      <ChevronLeftIcon className="size-4" />
      {backLabel(target)}
    </Link>
  );
}
