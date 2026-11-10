import Link from 'next/link';
import { Badge, EmptyState, Section, Stars } from '@/components/ui';
import { formatDate } from '@/lib/format';
import { profileHref, projectHref } from '@/lib/nav';
import type {
  HistoryItem,
  PersonRef,
  ProfileHistory,
  ProfileRatings,
  TeammateRating,
} from '@/lib/types';

const FORMER_STUDENT = 'Former student';

/** Closed accounts have no profile page, so their name is plain text. */
function PersonName({ person, from }: { person: PersonRef; from: string }) {
  if (person.displayName === FORMER_STUDENT) return <>{person.displayName}</>;
  return (
    <Link href={profileHref(person.id, from)} className="link">
      {person.displayName}
    </Link>
  );
}

const ENDED_NOTE: Record<string, string> = {
  left: 'Left the team',
  removed: 'Removed from the team',
  account_closed: 'Account closed',
};

function HistoryRow({ item, from }: { item: HistoryItem; from: string }) {
  const cancelled = item.project.status === 'cancelled';
  const note = item.leftReason ? ENDED_NOTE[item.leftReason] : null;
  return (
    <li className="py-3 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-center gap-2">
        <Link
          href={projectHref(item.project.id, from)}
          className="link font-medium"
        >
          {item.project.name}
        </Link>
        {cancelled ? <Badge tone="danger">Cancelled</Badge> : null}
      </div>
      <p className="text-sm text-ink-muted">{item.roleName}</p>
      <p className="text-sm text-ink-subtle">
        {formatDate(item.joinedAt)}
        {item.endedAt ? ` to ${formatDate(item.endedAt)}` : ' to now'}
        {note ? `. ${note}` : ''}
      </p>
    </li>
  );
}

function HistoryGroup({
  title,
  items,
  from,
}: {
  title: string;
  items: HistoryItem[];
  from: string;
}) {
  if (items.length === 0) return null;
  return (
    <div>
      <h3 className="mb-2 text-sm font-semibold text-ink-muted">{title}</h3>
      <ul className="divide-y divide-line">
        {items.map((item) => (
          <HistoryRow
            key={`${item.project.id}-${item.joinedAt}`}
            item={item}
            from={from}
          />
        ))}
      </ul>
    </div>
  );
}

export function HistorySection({
  history,
  from,
}: {
  history: ProfileHistory;
  from: string;
}) {
  const empty = history.current.length === 0 && history.past.length === 0;
  return (
    <Section title="Project history">
      {empty ? (
        <EmptyState
          title="No projects yet"
          description="Projects show here once this student joins a team."
        />
      ) : (
        <div className="space-y-6">
          <HistoryGroup title="Current" items={history.current} from={from} />
          <HistoryGroup title="Past" items={history.past} from={from} />
        </div>
      )}
    </Section>
  );
}

function teammateScore(entry: TeammateRating) {
  if (entry.state === 'published' && entry.score !== null) {
    return (
      <span className="inline-flex items-center gap-1.5 text-sm">
        <Stars value={entry.score} />
        <span className="font-medium">{entry.score}</span>
      </span>
    );
  }
  return (
    <span className="text-sm text-ink-subtle">
      {entry.state === 'pending' ? 'Ratings pending' : 'Not rated'}
    </span>
  );
}

export function TeammatesSection({
  teammates,
  from,
}: {
  teammates: TeammateRating[];
  from: string;
}) {
  return (
    <Section
      title="Past teammates"
      description="The score each teammate gave this student."
    >
      {teammates.length === 0 ? (
        <p className="text-ink-subtle">No past teammates yet.</p>
      ) : (
        <ul className="divide-y divide-line">
          {teammates.map((entry) => (
            <li
              key={`${entry.teammate.id}-${entry.project.id}`}
              className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 py-3 first:pt-0 last:pb-0"
            >
              <div className="min-w-0">
                <p className="font-medium">
                  <PersonName person={entry.teammate} from={from} />
                </p>
                <p className="text-sm text-ink-muted">
                  <Link
                    href={projectHref(entry.project.id, from)}
                    className="link"
                  >
                    {entry.project.name}
                  </Link>
                </p>
              </div>
              {teammateScore(entry)}
            </li>
          ))}
        </ul>
      )}
    </Section>
  );
}

export function RatingsSection({
  ratings,
  from,
}: {
  ratings: ProfileRatings;
  from: string;
}) {
  return (
    <Section title="Ratings">
      {ratings.published.length === 0 ? (
        <p className="text-ink-subtle">No published ratings yet.</p>
      ) : (
        <ul className="divide-y divide-line">
          {ratings.published.map((entry) => (
            <li
              key={`${entry.reviewer.id}-${entry.project.id}`}
              className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 py-3 first:pt-0 last:pb-0"
            >
              <p className="min-w-0 text-sm text-ink-muted">
                <span className="font-medium text-ink">
                  <PersonName person={entry.reviewer} from={from} />
                </span>{' '}
                on{' '}
                <Link
                  href={projectHref(entry.project.id, from)}
                  className="link"
                >
                  {entry.project.name}
                </Link>
              </p>
              <span className="inline-flex items-center gap-1.5 text-sm">
                <Stars value={entry.score} />
                <span className="font-medium">{entry.score}</span>
              </span>
            </li>
          ))}
        </ul>
      )}
      {ratings.pendingProjects.length > 0 ? (
        <p className="mt-3 text-sm text-ink-subtle">
          Ratings pending for{' '}
          {ratings.pendingProjects.map((project) => project.name).join(', ')}.
          They appear when the 14-day rating window closes.
        </p>
      ) : null}
    </Section>
  );
}
