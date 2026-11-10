import { Avatar, ButtonLink, Card, RatingSummary } from '@/components/ui';
import { ExpiresIn } from '@/features/requests/ExpiresIn';
import { RequestActions } from '@/features/requests/RequestActions';
import { facultyLine } from '@/lib/format';
import { profileHref } from '@/lib/nav';
import type { RecruitmentRequest } from '@/lib/types';

/** One pending application, for the lead to decide on. */
export function ApplicantCard({
  request,
  from,
}: {
  request: RecruitmentRequest;
  from: string;
}) {
  const { student } = request;
  return (
    <Card className="space-y-3">
      <div className="flex items-start gap-3">
        <Avatar name={student.displayName} />
        <div className="min-w-0 flex-1">
          <h3 className="font-semibold">{student.displayName}</h3>
          <p className="text-sm text-ink-muted">
            {facultyLine(student.faculty, student.year)}
          </p>
          <RatingSummary rating={student.rating} />
        </div>
      </div>
      <p className="text-sm">
        Applied for <strong className="font-medium">{request.role.name}</strong>
        .{' '}
        <span className="text-ink-muted">
          <ExpiresIn expiresAt={request.expiresAt} />
        </span>
      </p>
      {request.message ? (
        <blockquote className="whitespace-pre-line rounded-lg bg-sunken px-3 py-2 text-sm">
          {request.message}
        </blockquote>
      ) : null}
      <div className="flex flex-wrap items-start justify-between gap-2">
        <ButtonLink
          variant="secondary"
          size="sm"
          href={profileHref(student.id, from)}
        >
          Profile
        </ButtonLink>
        <RequestActions request={request} />
      </div>
    </Card>
  );
}
