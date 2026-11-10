import Link from 'next/link';
import { Alert, ButtonLink, Section, Stars } from '@/components/ui';
import { apiGet } from '@/lib/api';
import { formatDateTime } from '@/lib/format';
import { profileHref } from '@/lib/nav';
import type { ProjectDetail, ProjectRatings } from '@/lib/types';

export function PublishedRatings({
  ratings,
  project,
}: {
  ratings: ProjectRatings;
  project: ProjectDetail;
}) {
  const from = `/projects/${project.id}`;
  if (ratings.open)
    return (
      <Alert>
        Ratings pending. Scores stay private until{' '}
        {formatDateTime(ratings.closesAt)}.
      </Alert>
    );
  if (!ratings.published.length)
    return (
      <p className="text-sm text-ink-muted">
        No ratings were submitted. Missing ratings do not count as zero.
      </p>
    );
  return (
    <ul className="divide-y divide-line">
      {ratings.published.map((rating) => (
        <li
          key={`${rating.reviewer.id}-${rating.recipient.id}`}
          className="space-y-1 py-3 first:pt-0 last:pb-0"
        >
          <p className="text-sm">
            <Link
              href={profileHref(rating.reviewer.id, from)}
              className="font-medium underline underline-offset-2"
            >
              {rating.reviewer.displayName}
            </Link>{' '}
            rated{' '}
            <Link
              href={profileHref(rating.recipient.id, from)}
              className="font-medium underline underline-offset-2"
            >
              {rating.recipient.displayName}
            </Link>
          </p>
          <Stars value={rating.score} />
          <p className="text-sm text-ink-muted">{project.name}</p>
        </li>
      ))}
    </ul>
  );
}

export async function RatingsSummary({ project }: { project: ProjectDetail }) {
  if (project.status !== 'completed') return null;
  const ratings = await apiGet<ProjectRatings>(
    `/projects/${project.id}/ratings`,
  );
  return (
    <Section
      title="Teammate ratings"
      description={
        ratings.open
          ? 'Eligible members have 14 days after completion to rate each other.'
          : 'The rating window has closed. Published scores are read only.'
      }
      action={
        ratings.eligible ? (
          <ButtonLink
            href={`/projects/${project.id}/rate`}
            variant="secondary"
            size="sm"
          >
            {ratings.open ? 'Rate teammates' : 'View ratings'}
          </ButtonLink>
        ) : undefined
      }
    >
      <PublishedRatings ratings={ratings} project={project} />
    </Section>
  );
}
