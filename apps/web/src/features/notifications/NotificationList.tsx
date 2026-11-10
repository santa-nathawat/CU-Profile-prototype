'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { formatDateTime } from '@/lib/format';
import { clientApi } from '@/lib/api-client';
import type { Notification } from '@/lib/types';

function markRead(id: string): Promise<void> {
  return clientApi.post(`/notifications/${id}/read`).catch(() => undefined);
}

function NotificationRow({ item }: { item: Notification }) {
  const router = useRouter();
  const pathname = usePathname();
  const unread = item.readAt === null;

  return (
    <li>
      <Link
        href={item.link}
        onClick={async (event) => {
          if (!unread) return;
          if (event.metaKey || event.ctrlKey || event.shiftKey) {
            void markRead(item.id);
            return;
          }
          // Wait, so the next page and the bell see it as read.
          event.preventDefault();
          await markRead(item.id);
          // Some items, like a deleted team, have no page but the Inbox itself.
          if (item.link.split('?')[0] === pathname) router.refresh();
          else router.push(item.link);
        }}
        className={`flex items-start gap-3 px-4 py-3.5 transition-colors hover:bg-sunken sm:px-5 ${
          unread ? 'bg-accent-soft/60' : ''
        }`}
      >
        <span
          className={`mt-2 size-2 shrink-0 rounded-full ${
            unread ? 'bg-accent' : 'bg-transparent'
          }`}
          aria-hidden="true"
        />
        <span className="min-w-0">
          <span
            className={`block ${unread ? 'font-medium' : 'text-ink-muted'}`}
          >
            {unread ? <span className="sr-only">Unread: </span> : null}
            {item.message}
          </span>
          <span className="block text-sm text-ink-subtle">
            {formatDateTime(item.createdAt)}
          </span>
        </span>
      </Link>
    </li>
  );
}

export function NotificationList({ items }: { items: Notification[] }) {
  return (
    <ul className="divide-y divide-line overflow-hidden rounded-xl border border-line bg-surface">
      {items.map((item) => (
        <NotificationRow key={item.id} item={item} />
      ))}
    </ul>
  );
}
