import Link from 'next/link';
import { Badge, Card } from '@/components/ui';
import { requestHref } from '@/lib/nav';
import type { RecruitmentRequest } from '@/lib/types';
import { ExpiresIn } from './ExpiresIn';
import { RequestActions } from './RequestActions';
import { RequestFollowUps } from './RequestFollowUps';
import { RequestStatusBadge } from './RequestStatusBadge';
import { cancelReasonText, requestHeadline } from './request-status';

export function RequestRow({
  request,
  from,
}: {
  request: RecruitmentRequest;
  from: string;
}) {
  return (
    <Card className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <h2 className="font-semibold">
            <Link
              href={requestHref(request.id, from)}
              className="underline underline-offset-2 hover:text-accent-ink"
            >
              {request.project.name}
            </Link>
          </h2>
          <p className="text-sm text-ink-muted">{requestHeadline(request)}</p>
        </div>
        <div className="flex flex-wrap gap-1.5">
          <Badge>{request.project.category.name}</Badge>
          <RequestStatusBadge request={request} />
        </div>
      </div>
      {request.status === 'pending' ? (
        <p className="text-sm text-ink-muted">
          <ExpiresIn expiresAt={request.expiresAt} />
        </p>
      ) : null}
      {request.status === 'cancelled' ? (
        <p className="text-sm text-ink-muted">
          {cancelReasonText(request.cancelReason)}
        </p>
      ) : null}
      <RequestActions request={request} />
      <RequestFollowUps request={request} from={from} />
    </Card>
  );
}
