import { Section } from '@/components/ui';
import type { ProjectDetail, RecruitmentRequest } from '@/lib/types';
import { LeadTabs, type LeadTab } from './LeadTabs';
import { ProjectAbout } from '../ProjectOverview';
import { projectEnded } from '../project-rules';
import { RecruitmentToggle } from '../recruiting/RecruitmentToggle';
import { RoleManager } from '../posting/RoleManager';
import { RoleSummary } from '../RoleSummary';
import { ContactLinksEditor } from './ContactLinks';
import { LifecycleActions } from './LifecycleActions';
import { Transfer } from './TeamActions';
import { RatingsSummary } from '../ratings/RatingsSummary';

export function LeadView({
  project,
  requests,
  tab,
  backTo,
  here,
}: {
  project: ProjectDetail;
  requests: RecruitmentRequest[];
  tab: LeadTab;
  backTo: string | undefined;
  here: string;
}) {
  const pending = requests.filter((request) => request.status === 'pending');
  const pendingByRole: Record<string, number> = {};
  for (const request of pending) {
    pendingByRole[request.role.id] = (pendingByRole[request.role.id] ?? 0) + 1;
  }

  return (
    <div className="mt-6 space-y-4">
      <LifecycleActions project={project} pendingCount={pending.length} />
      <RatingsSummary project={project} />
      <ProjectAbout project={project} />
      <Section title="Recruitment">
        <RecruitmentToggle project={project} pendingCount={pending.length} />
      </Section>
      <Section
        title="Roles"
        description="You can't go below the number of members already in a role."
      >
        {projectEnded(project.status) ? (
          <RoleSummary roles={project.roles} />
        ) : (
          <RoleManager
            projectId={project.id}
            roles={project.roles}
            pendingByRole={pendingByRole}
          />
        )}
      </Section>
      <ContactLinksEditor project={project} />
      <Transfer project={project} />
      <LeadTabs
        project={project}
        requests={requests}
        tab={tab}
        backTo={backTo}
        here={here}
      />
    </div>
  );
}
