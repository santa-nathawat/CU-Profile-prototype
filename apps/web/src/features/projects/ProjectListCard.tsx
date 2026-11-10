import Link from 'next/link';
import { Badge, RatingSummary } from '@/components/ui';
import { formatDuration, plural } from '@/lib/format';
import { projectHref } from '@/lib/nav';
import type { ProjectCard } from '@/lib/types';
import { placesLeft } from './project-rules';

/** One project in Discover. `from` is the list's URL, for the team page's Back. */
export function ProjectListCard({
  project,
  from,
  membership = null,
}: {
  project: ProjectCard;
  from: string;
  /** Search includes the viewer's own teams, which they can't apply to. */
  membership?: 'lead' | 'member' | null;
}) {
  const open = project.placesTotal - project.placesTaken;
  return (
    <Link
      href={projectHref(project.id, from)}
      className="flex h-full flex-col gap-3 rounded-xl border border-line bg-surface p-5 transition-colors hover:border-line-strong"
    >
      <div>
        <div className="flex flex-wrap gap-1.5">
          <Badge>{project.category.name}</Badge>
          {membership ? (
            <Badge tone="accent">
              {membership === 'lead' ? 'You lead this team' : 'Your team'}
            </Badge>
          ) : null}
        </div>
        <h2 className="mt-2 font-semibold underline underline-offset-2">
          {project.name}
        </h2>
        <p className="mt-1 line-clamp-2 text-sm text-ink-muted">
          {project.description}
        </p>
      </div>
      <p className="text-sm text-ink-subtle">
        {formatDuration(project.durationValue, project.durationUnit)},{' '}
        {plural(open, 'place')} open
      </p>
      <ul className="space-y-1 text-sm" aria-label="Roles">
        {project.roles.map((role) => (
          <li key={role.id} className="flex justify-between gap-3">
            <span className="min-w-0">{role.name}</span>
            <span
              className={`shrink-0 ${
                role.available > 0 ? 'font-medium' : 'text-ink-subtle'
              }`}
            >
              {placesLeft(role)}
            </span>
          </li>
        ))}
      </ul>
      <div className="mt-auto border-t border-line pt-3 text-sm">
        <p>
          <span className="text-ink-subtle">Lead: </span>
          {project.lead.displayName}
        </p>
        <RatingSummary rating={project.lead.rating} className="mt-0.5" />
      </div>
    </Link>
  );
}
