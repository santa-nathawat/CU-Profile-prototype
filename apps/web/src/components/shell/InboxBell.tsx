'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { usePathname } from 'next/navigation';
import { clientApi } from '@/lib/api-client';
import { reloadTo } from '@/lib/nav';
import { ApiError } from '@/lib/api-error';
import type { Me } from '@/lib/types';
import { BellIcon } from '@/components/ui/icons';

/**
 * Layouts don't re-render on client navigation, so the count is re-read on each
 * route change. That also catches a session that ended in the meantime.
 */
export function InboxBell({ unread }: { unread: number }) {
  const pathname = usePathname();
  const [live, setLive] = useState<{ count: number; base: number } | null>(
    null,
  );

  useEffect(() => {
    let stale = false;
    clientApi
      .get<Me>('/me')
      .then((me) => {
        if (!stale) setLive({ count: me.unreadNotifications, base: unread });
      })
      .catch((error) => {
        if (error instanceof ApiError && error.status === 401) {
          reloadTo('/sign-in');
        }
      });
    return () => {
      stale = true;
    };
  }, [pathname, unread]);

  // A newer count from the server (router.refresh) replaces the fetched one.
  const count = live && live.base === unread ? live.count : unread;
  const active = pathname === '/notifications';

  return (
    <Link
      href="/notifications"
      aria-label={count > 0 ? `Inbox, ${count} unread` : 'Inbox'}
      aria-current={active ? 'page' : undefined}
      className={`relative inline-flex size-11 items-center justify-center rounded-lg hover:bg-sunken ${
        active ? 'text-accent-ink' : 'text-ink-muted hover:text-ink'
      }`}
    >
      <BellIcon />
      {count > 0 ? (
        <span
          className="absolute top-1 right-0.5 min-w-4.5 rounded-full bg-accent px-1 text-center text-xs font-semibold text-white"
          aria-hidden="true"
        >
          {count > 99 ? '99+' : count}
        </span>
      ) : null}
    </Link>
  );
}
