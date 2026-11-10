import Link from 'next/link';
import { Card } from '@/components/ui';
import { ExpiresIn } from '@/features/requests/ExpiresIn';
import { RequestActions } from '@/features/requests/RequestActions';
import { RequestStatusBadge } from '@/features/requests/RequestStatusBadge';
import { requestHref } from '@/lib/nav';
import type { RecruitmentRequest } from '@/lib/types';

/** A visitor's open application or invitation for this team. */
export function PendingRequestCard({
  request,
  from,
}: {
  request: RecruitmentRequest;
  from: string;
}) {
  const invited = request.type === 'invitation';
  const lead = request.invitedBy?.displayName ?? 'The lead';
  return (
    <Card className="space-y-3 border-accent/40 bg-accent-soft/40">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <h2 className="text-lg font-semibold">
          {invited
            ? `${lead} invited you as ${request.role.name}`
            : `You applied as ${request.role.name}`}
        </h2>
        <RequestStatusBadge request={request} />
      </div>
      <p className="text-sm text-ink-muted">
        <ExpiresIn expiresAt={request.expiresAt} withDate />
      </p>
      {request.message ? (
        <p className="whitespace-pre-line text-sm">{request.message}</p>
      ) : null}
      <RequestActions request={request} />
      <p className="text-sm">
        <Link href={requestHref(request.id, from)} className="link">
          See request details
        </Link>
      </p>
    </Card>
  );
}
