import type { Metadata } from 'next';
import { PageHeader } from '@/components/PageHeader';
import { EmptyState, Pagination, Tabs } from '@/components/ui';
import { MarkAllRead } from '@/features/notifications/MarkAllRead';
import { NotificationList } from '@/features/notifications/NotificationList';
import { apiGet } from '@/lib/api';
import { firstParam, listHref, parsePage } from '@/lib/nav';
import type { NotificationPage } from '@/lib/types';

export const metadata: Metadata = { title: 'Inbox' };

const PAGE_SIZE = 20;
const PATH = '/notifications';

export default async function NotificationsPage(
  props: PageProps<'/notifications'>,
) {
  const params = await props.searchParams;
  const unreadOnly = firstParam(params.filter) === 'unread';
  const page = parsePage(params.page);

  const data = await apiGet<NotificationPage>(PATH, {
    unread: unreadOnly ? 'true' : undefined,
    page,
    pageSize: PAGE_SIZE,
  });

  const filter = unreadOnly ? 'unread' : undefined;

  return (
    <div className="mx-auto max-w-3xl">
      <PageHeader
        title="Inbox"
        action={data.unreadCount > 0 ? <MarkAllRead /> : undefined}
      />
      <Tabs
        label="Filter notifications"
        items={[
          { label: 'All', active: !unreadOnly, href: PATH },
          {
            label: 'Unread',
            count: data.unreadCount,
            active: unreadOnly,
            href: listHref(PATH, { filter: 'unread' }),
          },
        ]}
      />
      <div className="mt-4">
        {data.items.length > 0 ? (
          <NotificationList items={data.items} />
        ) : (
          <EmptyState
            title={unreadOnly ? 'No unread notifications' : 'Nothing here yet'}
            description={
              unreadOnly
                ? "You're all caught up."
                : 'Applications, invitations and team updates will show here.'
            }
          />
        )}
      </div>
      <Pagination
        page={data.page}
        pageSize={data.pageSize}
        total={data.total}
        hrefForPage={(next) => listHref(PATH, { filter, page: next })}
      />
      <p className="mt-8 text-sm text-ink-subtle">
        Activity shows up here in your Inbox. Activity email and push come
        later.
      </p>
    </div>
  );
}
