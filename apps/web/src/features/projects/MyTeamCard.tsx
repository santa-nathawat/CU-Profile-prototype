import Link from 'next/link';
import { Alert, Badge, Card } from '@/components/ui';
import { projectHref } from '@/lib/nav';
import type { MyProject } from '@/lib/types';
import { ProjectBadges } from './ProjectBadges';

export function MyTeamCard({
  project,
  from,
}: {
  project: MyProject;
  from: string;
}) {
  return (
    <Card className="flex h-full flex-col gap-3">
      <ProjectBadges project={project} />
      <div>
        <h3 className="font-semibold">
          <Link
            href={projectHref(project.id, from)}
            className="underline underline-offset-2 hover:text-accent-ink"
          >
            {project.name}
          </Link>
        </h3>
        <p className="flex flex-wrap items-center gap-1.5 text-sm text-ink-muted">
          <span>Your role: {project.roleName}</span>
          {project.isLead ? <Badge tone="accent">Lead</Badge> : null}
        </p>
        <p className="text-sm text-ink-muted">
          {project.placesTaken} of {project.placesTotal} places taken
        </p>
      </div>
      {project.hasPendingTransferToMe ? (
        <Alert tone="warning">The lead asked you to take over this team.</Alert>
      ) : null}
      {project.ratingsOpen ? (
        <Alert>Ratings are open for this project.</Alert>
      ) : null}
    </Card>
  );
}
