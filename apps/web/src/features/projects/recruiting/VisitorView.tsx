import { Section } from '@/components/ui';
import type { ProjectDetail } from '@/lib/types';
import { ApplyRoles } from './ApplyRoles';
import { MemberList } from '../team/MemberList';
import { PendingRequestCard } from './PendingRequestCard';
import { LeadCard, ProjectAbout } from '../ProjectOverview';
import { RatingsSummary } from '../ratings/RatingsSummary';

export function VisitorView({
  project,
  from,
}: {
  project: ProjectDetail;
  from: string;
}) {
  const pending = project.viewer.pendingRequest;
  return (
    <div className="mt-6 space-y-4">
      {pending ? <PendingRequestCard request={pending} from={from} /> : null}
      <RatingsSummary project={project} />
      <ProjectAbout project={project} />
      <LeadCard project={project} from={from} />
      <Section title="Roles">
        <ApplyRoles project={project} />
      </Section>
      <Section title="Members">
        <MemberList members={project.members} from={from} />
      </Section>
    </div>
  );
}
