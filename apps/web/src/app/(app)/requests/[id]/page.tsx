import type { Metadata } from 'next';
import Link from 'next/link';
import { BackLink } from '@/components/BackLink';
import {
  Alert,
  ButtonLink,
  Facts,
  RatingSummary,
  Section,
} from '@/components/ui';
import { MemberList } from '@/features/projects/team/MemberList';
import { ExpiresIn } from '@/features/requests/ExpiresIn';
import { RequestActions } from '@/features/requests/RequestActions';
import { RequestFollowUps } from '@/features/requests/RequestFollowUps';
import { RequestStatusBadge } from '@/features/requests/RequestStatusBadge';
import {
  cancelReasonText,
  requestHeadline,
} from '@/features/requests/request-status';
import { apiGet, apiGetOrNotFound } from '@/lib/api';
import { ApiError } from '@/lib/api-error';
import { facultyLine, formatDateTime, formatDuration } from '@/lib/format';
import { profileHref, projectHref, requestHref, safeFrom } from '@/lib/nav';
import type { ProjectDetail, RecruitmentRequest } from '@/lib/types';

export const metadata: Metadata = { title: 'Request' };

/** A request outlives its team, and a deleted team has no page to read. */
async function loadProject(id: string): Promise<ProjectDetail | null> {
  try {
    return await apiGet<ProjectDetail>(`/projects/${id}`);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return null;
    throw error;
  }
}

function deadlineFact(request: RecruitmentRequest) {
  if (request.status === 'pending') {
    return <ExpiresIn expiresAt={request.expiresAt} withDate />;
  }
  if (request.status === 'expired') {
    return `Expired ${formatDateTime(request.expiresAt)}`;
  }
  return request.resolvedAt ? formatDateTime(request.resolvedAt) : null;
}

export default async function RequestPage(props: PageProps<'/requests/[id]'>) {
  const { id } = await props.params;
  const from = safeFrom((await props.searchParams).from);

  const request = await apiGetOrNotFound<RecruitmentRequest>(`/requests/${id}`);
  const project = await loadProject(request.project.id);

  const here = requestHref(id, from ?? undefined);
  const { viewer, student } = request;
  const invitationToMe = viewer.isStudent && request.type === 'invitation';
  const open = request.status === 'pending';

  const deadline = deadlineFact(request);
  const facts = [
    { label: 'Role', value: request.role.name },
    { label: 'Category', value: request.project.category.name },
    ...(project
      ? [
          {
            label: 'Expected duration',
            value: formatDuration(project.durationValue, project.durationUnit),
          },
          {
            label: 'Places taken',
            value: `${project.placesTaken} of ${project.placesTotal}`,
          },
        ]
      : []),
    { label: 'Sent', value: formatDateTime(request.createdAt) },
    ...(deadline
      ? [
          {
            label:
              open || request.status === 'expired' ? 'Deadline' : 'Answered',
            value: deadline,
          },
        ]
      : []),
  ];

  return (
    <div className="mx-auto max-w-3xl">
      <BackLink from={from} fallback="/requests" />

      <header className="mt-3">
        <RequestStatusBadge request={request} />
        <h1 className="mt-2 text-2xl font-semibold">
          {!project ? (
            request.project.name
          ) : (
            <Link
              href={projectHref(request.project.id, here)}
              className="underline-offset-2 hover:underline"
            >
              {request.project.name}
            </Link>
          )}
        </h1>
        <p className="text-ink-muted">{requestHeadline(request)}</p>
      </header>

      <div className="mt-6 space-y-4">
        {request.status === 'cancelled' ? (
          <Alert>{cancelReasonText(request.cancelReason)}</Alert>
        ) : null}

        <Section title="Request details">
          <Facts items={facts} />
          {request.message ? (
            <div className="mt-4">
              <p className="text-sm text-ink-subtle">Message to the lead</p>
              <p className="whitespace-pre-line">{request.message}</p>
            </div>
          ) : null}
          {project ? (
            <p className="mt-4 whitespace-pre-line text-ink-muted">
              {project.description}
            </p>
          ) : null}
        </Section>

        {invitationToMe && (open || request.status === 'expired') ? (
          <p className="text-sm text-ink-muted">
            Accepting joins the team at once, and your other requests stay as
            they are. If you decline, the lead can&apos;t invite you again until
            you apply to this team yourself.
          </p>
        ) : null}
        <RequestActions
          request={request}
          size="md"
          showExpired
          openTeamOnJoin
        />
        {project || request.viewer.isStudent ? (
          <RequestFollowUps
            request={request}
            from={here}
            teamExists={project !== null}
          />
        ) : null}

        {viewer.isLead ? (
          <Section title="Student">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="min-w-0">
                <p className="font-medium">{student.displayName}</p>
                <p className="text-sm text-ink-muted">
                  {facultyLine(student.faculty, student.year)}
                </p>
                <RatingSummary rating={student.rating} />
              </div>
              <ButtonLink
                variant="secondary"
                size="sm"
                href={profileHref(student.id, here)}
              >
                Profile
              </ButtonLink>
            </div>
          </Section>
        ) : null}

        {project && !viewer.isLead ? (
          <Section title="Team lead">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="min-w-0">
                <p className="font-medium">{project.lead.displayName}</p>
                <RatingSummary rating={project.lead.rating} />
              </div>
              <ButtonLink
                variant="secondary"
                size="sm"
                href={profileHref(project.lead.id, here)}
              >
                View lead profile
              </ButtonLink>
            </div>
          </Section>
        ) : null}

        {project ? (
          <Section title="Members">
            <MemberList members={project.members} from={here} />
          </Section>
        ) : null}
      </div>
    </div>
  );
}
