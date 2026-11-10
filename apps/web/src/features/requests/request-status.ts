import type { BadgeTone } from '@/components/ui';
import { projectEnded } from '@/features/projects/project-rules';
import type {
  CancelReason,
  RecruitmentRequest,
  RequestStatus,
} from '@/lib/types';

export const STATUS: Record<RequestStatus, { label: string; tone: BadgeTone }> =
  {
    pending: { label: 'Pending', tone: 'warning' },
    approved: { label: 'Approved', tone: 'success' },
    accepted: { label: 'Accepted', tone: 'success' },
    rejected: { label: 'Rejected', tone: 'danger' },
    declined: { label: 'Declined', tone: 'neutral' },
    withdrawn: { label: 'Withdrawn', tone: 'neutral' },
    cancelled: { label: 'Cancelled', tone: 'neutral' },
    expired: { label: 'Expired', tone: 'neutral' },
  };

const CANCEL_REASONS: Record<CancelReason, string> = {
  lead_cancelled: 'The lead cancelled this invitation.',
  role_removed: 'The lead removed this role from the team.',
  role_full: 'This role filled up before the request was answered.',
  recruitment_closed: 'The lead closed recruitment for this team.',
  project_completed: 'The project was completed.',
  project_cancelled: 'The project was cancelled.',
  team_deleted: 'The lead deleted this team.',
  account_closed: 'The student closed their account.',
};

export function cancelReasonText(reason: CancelReason | null): string {
  return reason ? CANCEL_REASONS[reason] : 'This request was cancelled.';
}

/** One line saying who asked whom, from the viewer's side. */
export function requestHeadline(request: RecruitmentRequest): string {
  const role = request.role.name;
  if (request.viewer.isStudent) {
    if (request.type === 'application') return `You applied as ${role}`;
    const lead = request.invitedBy?.displayName ?? 'The lead';
    return `${lead} invited you as ${role}`;
  }
  const student = request.student.displayName;
  if (request.type === 'application') return `${student} applied as ${role}`;
  return `${student} was invited as ${role}`;
}

export function teamGone(request: RecruitmentRequest): boolean {
  return request.cancelReason === 'team_deleted';
}

/** Students may try again unless the lead rejected them or the team is over. */
export function canApplyAgain(request: RecruitmentRequest): boolean {
  if (!request.viewer.isStudent) return false;
  if (teamGone(request) || projectEnded(request.project.status)) return false;
  return ['withdrawn', 'cancelled', 'expired', 'declined'].includes(
    request.status,
  );
}
