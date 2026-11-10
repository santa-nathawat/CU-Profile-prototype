import {
  Avatar,
  ButtonLink,
  Facts,
  RatingSummary,
  Section,
} from '@/components/ui';
import { formatDate, formatDuration } from '@/lib/format';
import { profileHref } from '@/lib/nav';
import type { ProjectDetail } from '@/lib/types';
import { ProjectBadges } from './ProjectBadges';

export function ProjectHeader({
  project,
  action,
}: {
  project: ProjectDetail;
  action?: React.ReactNode;
}) {
  return (
    <header className="mt-3 flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0">
        <ProjectBadges project={project} />
        <h1 className="mt-2 text-2xl font-semibold">{project.name}</h1>
        <p className="text-ink-muted">
          {project.placesTaken} of {project.placesTotal} places taken
        </p>
      </div>
      {action}
    </header>
  );
}

export function ProjectAbout({ project }: { project: ProjectDetail }) {
  const facts = [
    { label: 'Category', value: project.category.name },
    {
      label: 'Expected duration',
      value: formatDuration(project.durationValue, project.durationUnit),
    },
    {
      label: 'Places taken',
      value: `${project.placesTaken} of ${project.placesTotal}`,
    },
  ];
  if (project.startedAt) {
    facts.push({ label: 'Started', value: formatDate(project.startedAt) });
  }
  if (project.expectedEndDate) {
    facts.push({
      label: 'Expected end date',
      value: formatDate(project.expectedEndDate),
    });
  }
  if (project.completedAt) {
    facts.push({ label: 'Completed', value: formatDate(project.completedAt) });
  }
  if (project.cancelledAt) {
    facts.push({ label: 'Cancelled', value: formatDate(project.cancelledAt) });
  }

  return (
    <Section title="About this project">
      <p className="whitespace-pre-line">{project.description}</p>
      <Facts items={facts} className="mt-4" />
    </Section>
  );
}

export function LeadCard({
  project,
  from,
}: {
  project: ProjectDetail;
  from: string;
}) {
  const { lead } = project;
  return (
    <Section title="Team lead">
      <div className="flex flex-wrap items-center gap-3">
        <Avatar name={lead.displayName} />
        <div className="min-w-0 flex-1">
          <p className="font-medium">{lead.displayName}</p>
          <RatingSummary rating={lead.rating} />
        </div>
        <ButtonLink
          variant="secondary"
          size="sm"
          href={profileHref(lead.id, from)}
        >
          View profile
        </ButtonLink>
      </div>
    </Section>
  );
}
