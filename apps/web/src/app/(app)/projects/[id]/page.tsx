import type { Metadata } from 'next';
import { BackLink } from '@/components/BackLink';
import { ButtonLink } from '@/components/ui';
import { EditPostAction } from '@/features/projects/posting/EditPostAction';
import { parseLeadTab } from '@/features/projects/team/LeadTabs';
import { LeadView } from '@/features/projects/team/LeadView';
import { MemberView } from '@/features/projects/team/MemberView';
import { ProjectHeader } from '@/features/projects/ProjectOverview';
import { projectEnded } from '@/features/projects/project-rules';
import { VisitorView } from '@/features/projects/recruiting/VisitorView';
import { apiGet, apiGetOrNotFound } from '@/lib/api';
import { requireStudent } from '@/lib/auth';
import { firstParam, listHref, safeFrom } from '@/lib/nav';
import type {
  Category,
  ItemList,
  ProjectDetail,
  RecruitmentRequest,
} from '@/lib/types';

export const metadata: Metadata = { title: 'Project' };

export default async function ProjectPage(props: PageProps<'/projects/[id]'>) {
  const { id } = await props.params;
  const params = await props.searchParams;
  const from = safeFrom(params.from) ?? undefined;

  const me = await requireStudent();
  const project = await apiGetOrNotFound<ProjectDetail>(`/projects/${id}`);
  const { isLead, isMember } = project.viewer;

  const path = `/projects/${id}`;
  const fallback = isLead || isMember ? '/my-teams' : '/projects';

  if (!isLead) {
    const here = listHref(path, { from });
    return (
      <div className="mx-auto max-w-4xl">
        <BackLink from={from ?? null} fallback={fallback} />
        <ProjectHeader project={project} />
        {isMember ? (
          <MemberView project={project} viewerId={me.id} from={here} />
        ) : (
          <VisitorView project={project} from={here} />
        )}
      </div>
    );
  }

  const [{ items: requests }, { items: categories }] = await Promise.all([
    apiGet<ItemList<RecruitmentRequest>>(`${path}/requests`),
    apiGet<ItemList<Category>>('/categories'),
  ]);
  const tab = parseLeadTab(firstParam(params.tab));
  const here = listHref(path, {
    from,
    tab: tab === 'applicants' ? undefined : tab,
  });

  return (
    <div className="mx-auto max-w-4xl">
      <BackLink from={from ?? null} fallback={fallback} />
      <ProjectHeader
        project={project}
        action={
          <div className="flex flex-wrap gap-2">
            {projectEnded(project.status) ? null : (
              <>
                <ButtonLink href="/people" variant="secondary" size="sm">
                  Find people
                </ButtonLink>
                <EditPostAction project={project} categories={categories} />
              </>
            )}
          </div>
        }
      />
      <LeadView
        project={project}
        requests={requests}
        tab={tab}
        backTo={from}
        here={here}
      />
    </div>
  );
}
