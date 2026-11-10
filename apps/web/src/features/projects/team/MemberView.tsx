import { Section } from '@/components/ui';
import type { ProjectDetail } from '@/lib/types';
import { MemberList } from './MemberList';
import { LeadCard, ProjectAbout } from '../ProjectOverview';
import { RoleSummary } from '../RoleSummary';
import { ContactLinks } from './ContactLinks';
import { LeaveTeam, Transfer } from './TeamActions';
import { RatingsSummary } from '../ratings/RatingsSummary';

export function MemberView({
  project,
  viewerId,
  from,
}: {
  project: ProjectDetail;
  viewerId: string;
  from: string;
}) {
  const me = project.members.find((member) => member.userId === viewerId);
  return (
    <div className="mt-6 space-y-4">
      {me ? (
        <p className="text-lg font-medium">You&apos;re the {me.roleName}</p>
      ) : null}
      <Transfer project={project} />
      <RatingsSummary project={project} />
      <ContactLinks project={project} />
      <ProjectAbout project={project} />
      <LeadCard project={project} from={from} />
      <Section title="Members">
        <MemberList members={project.members} from={from} />
      </Section>
      <Section title="Roles">
        <RoleSummary roles={project.roles} />
      </Section>
      <LeaveTeam project={project} />
    </div>
  );
}
