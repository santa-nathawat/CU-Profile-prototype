import Link from 'next/link';
import { ExpiresIn } from '@/features/requests/ExpiresIn';
import { RequestActions } from '@/features/requests/RequestActions';
import { RequestStatusBadge } from '@/features/requests/RequestStatusBadge';
import { cancelReasonText } from '@/features/requests/request-status';
import { profileHref } from '@/lib/nav';
import type { RecruitmentRequest } from '@/lib/types';

/** One invitation the team sent. Cancel appears while it can still be cancelled. */
export function InviteRow({
  request,
  from,
}: {
  request: RecruitmentRequest;
  from: string;
}) {
  return (
    <li className="py-3 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <p className="flex flex-wrap items-center gap-2">
            <Link
              href={profileHref(request.student.id, from)}
              className="font-medium underline-offset-2 hover:underline"
            >
              {request.student.displayName}
            </Link>
            <RequestStatusBadge request={request} />
          </p>
          <p className="text-sm text-ink-muted">
            {request.role.name}
            {request.status === 'pending' ? (
              <>
                . <ExpiresIn expiresAt={request.expiresAt} />
              </>
            ) : null}
          </p>
          {request.status === 'cancelled' ? (
            <p className="text-sm text-ink-muted">
              {cancelReasonText(request.cancelReason)}
            </p>
          ) : null}
        </div>
        <RequestActions request={request} />
      </div>
    </li>
  );
}
