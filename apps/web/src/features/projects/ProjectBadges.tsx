import { Badge, type BadgeTone } from '@/components/ui';
import type { ProjectCard, ProjectStatus } from '@/lib/types';
import { projectEnded } from './project-rules';

const STATUS: Record<ProjectStatus, { label: string; tone: BadgeTone }> = {
  not_started: { label: 'Not started', tone: 'neutral' },
  in_progress: { label: 'In progress', tone: 'accent' },
  completed: { label: 'Completed', tone: 'success' },
  cancelled: { label: 'Cancelled', tone: 'danger' },
};

export function ProjectStatusBadge({ status }: { status: ProjectStatus }) {
  return <Badge tone={STATUS[status].tone}>{STATUS[status].label}</Badge>;
}

export function ProjectBadges({
  project,
}: {
  project: Pick<ProjectCard, 'status' | 'recruitmentStatus' | 'category'>;
}) {
  const recruiting = project.recruitmentStatus === 'open';
  return (
    <div className="flex flex-wrap gap-1.5">
      <Badge>{project.category.name}</Badge>
      {recruiting ? <Badge tone="success">Recruiting</Badge> : null}
      {!recruiting && !projectEnded(project.status) ? (
        <Badge tone="warning">Recruitment closed</Badge>
      ) : null}
      {recruiting && project.status === 'not_started' ? null : (
        <ProjectStatusBadge status={project.status} />
      )}
    </div>
  );
}
