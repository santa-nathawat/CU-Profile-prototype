import Link from 'next/link';
import { Avatar, Badge, RatingSummary } from '@/components/ui';
import { facultyLine } from '@/lib/format';
import { profileHref } from '@/lib/nav';
import type { InvitableProject, ProfileCard } from '@/lib/types';
import { InviteAction } from './InviteAction';

function matches(skill: string, query: string): boolean {
  return query !== '' && skill.toLowerCase().includes(query.toLowerCase());
}

/**
 * One person in a list. `from` is the list's URL, for the profile's Back link.
 * The name's link covers the whole card, leaving Invite clickable on top of it.
 */
export function PersonCard({
  person,
  from,
  highlightSkill = '',
  invitableProjects = [],
}: {
  person: ProfileCard;
  from: string;
  highlightSkill?: string;
  invitableProjects?: InvitableProject[];
}) {
  return (
    <article className="relative flex h-full flex-col gap-3 rounded-xl border border-line bg-surface p-5 transition-colors hover:border-line-strong">
      <div className="flex items-start gap-3">
        <Avatar name={person.displayName} />
        <div className="min-w-0">
          <h2 className="font-semibold">
            <Link
              href={profileHref(person.userId, from)}
              className="underline-offset-2 after:absolute after:inset-0 after:rounded-xl hover:underline"
            >
              {person.displayName}
            </Link>
          </h2>
          <p className="text-sm text-ink-muted">
            {facultyLine(person.faculty, person.year)}
          </p>
        </div>
      </div>
      {person.headline ? (
        <p className="line-clamp-2 text-sm text-ink-muted">{person.headline}</p>
      ) : null}
      <ul className="flex flex-wrap gap-1.5" aria-label="Top skills">
        {person.skills.map((skill) => (
          <li key={skill}>
            <Badge tone={matches(skill, highlightSkill) ? 'accent' : 'neutral'}>
              {skill}
            </Badge>
          </li>
        ))}
      </ul>
      <div className="mt-auto flex flex-wrap items-center justify-between gap-2 pt-1">
        <RatingSummary rating={person.rating} />
        {invitableProjects.length > 0 ? (
          <div className="relative z-10">
            <InviteAction
              studentId={person.userId}
              studentName={person.displayName}
              projects={invitableProjects}
              size="sm"
              label="Invite"
            />
          </div>
        ) : null}
      </div>
    </article>
  );
}
