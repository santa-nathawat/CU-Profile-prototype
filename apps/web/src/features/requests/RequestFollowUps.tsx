import { ButtonLink } from '@/components/ui';
import { projectHref } from '@/lib/nav';
import type { RecruitmentRequest } from '@/lib/types';
import { canApplyAgain } from './request-status';
import { SimilarTeams } from './SimilarTeams';

/** What to do next once a request has an outcome. */
export function RequestFollowUps({
  request,
  from,
  teamExists = true,
}: {
  request: RecruitmentRequest;
  from: string;
  /** The detail page knows a team was deleted even when the request doesn't say. */
  teamExists?: boolean;
}) {
  const { status, viewer, project } = request;
  const joined = teamExists && (status === 'approved' || status === 'accepted');
  const lookElsewhere =
    viewer.isStudent && (status === 'rejected' || status === 'expired');
  const applyAgain = teamExists && canApplyAgain(request);

  if (!joined && !lookElsewhere && !applyAgain) return null;

  return (
    <div className="flex flex-wrap items-center gap-2">
      {joined ? (
        <ButtonLink
          size="sm"
          variant="secondary"
          href={projectHref(project.id, from)}
        >
          Open team
        </ButtonLink>
      ) : null}
      {applyAgain ? (
        <ButtonLink
          size="sm"
          variant="secondary"
          href={projectHref(project.id, from)}
        >
          {request.type === 'application'
            ? 'Apply again'
            : 'Apply to this team'}
        </ButtonLink>
      ) : null}
      {lookElsewhere ? (
        <SimilarTeams projectId={project.id} from={from} />
      ) : null}
    </div>
  );
}
