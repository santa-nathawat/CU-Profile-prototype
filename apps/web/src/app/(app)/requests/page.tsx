import type { Metadata } from 'next';
import { PageHeader } from '@/components/PageHeader';
import { ButtonLink, EmptyState, Tabs } from '@/components/ui';
import { RequestRow } from '@/features/requests/RequestRow';
import { apiGet } from '@/lib/api';
import { firstParam, listHref } from '@/lib/nav';
import type { MyRequests, RecruitmentRequest } from '@/lib/types';

export const metadata: Metadata = { title: 'Requests' };

const PATH = '/requests';

function pendingCount(requests: RecruitmentRequest[]): number | undefined {
  const count = requests.filter((item) => item.status === 'pending').length;
  return count > 0 ? count : undefined;
}

/** Open requests need an answer, so they come first. Otherwise newest first. */
function pendingFirst(requests: RecruitmentRequest[]): RecruitmentRequest[] {
  return [
    ...requests.filter((item) => item.status === 'pending'),
    ...requests.filter((item) => item.status !== 'pending'),
  ];
}

export default async function RequestsPage(props: PageProps<'/requests'>) {
  const params = await props.searchParams;
  const received = firstParam(params.tab) === 'received';

  const data = await apiGet<MyRequests>('/me/requests');
  const items = pendingFirst(received ? data.received : data.sent);
  const here = listHref(PATH, { tab: received ? 'received' : undefined });

  return (
    <div className="mx-auto max-w-3xl">
      <PageHeader
        title="Requests"
        description="Applications and invitations you sent or received. Each one stays open for 48 hours."
      />
      <Tabs
        label="Sent or received requests"
        items={[
          {
            label: 'Sent',
            count: pendingCount(data.sent),
            active: !received,
            href: PATH,
          },
          {
            label: 'Received',
            count: pendingCount(data.received),
            active: received,
            href: listHref(PATH, { tab: 'received' }),
          },
        ]}
      />
      <div className="mt-4">
        {items.length > 0 ? (
          <ul className="space-y-3">
            {items.map((request) => (
              <li key={request.id}>
                <RequestRow request={request} from={here} />
              </li>
            ))}
          </ul>
        ) : received ? (
          <EmptyState
            title="Nothing received yet"
            description="Invitations to you and applications to teams you lead show here."
            action={
              <ButtonLink href="/settings" variant="secondary">
                Update your profile
              </ButtonLink>
            }
          />
        ) : (
          <EmptyState
            title="You haven't sent any requests"
            description="Applications you send and people you invite show here."
            action={
              <ButtonLink href="/projects" variant="secondary">
                Discover projects
              </ButtonLink>
            }
          />
        )}
      </div>
    </div>
  );
}
