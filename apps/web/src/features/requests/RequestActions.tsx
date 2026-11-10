'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Button, ConfirmDialog, FormAlert } from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import { showNotice } from '@/lib/notice';
import type { RecruitmentRequest, RequestViewer } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';
import { useNow } from '@/lib/use-now';

type Action =
  'cancel' | 'withdraw' | 'decline' | 'reject' | 'accept' | 'approve';

const LABELS: Record<Action, string> = {
  cancel: 'Cancel invitation',
  withdraw: 'Withdraw',
  decline: 'Decline',
  reject: 'Reject',
  accept: 'Accept',
  approve: 'Approve',
};

const PERMISSIONS: Record<Action, keyof RequestViewer> = {
  cancel: 'canCancel',
  withdraw: 'canWithdraw',
  decline: 'canDecline',
  reject: 'canReject',
  accept: 'canAccept',
  approve: 'canApprove',
};

// Least committing first, so the main action always sits on the right.
const ORDER: Action[] = [
  'cancel',
  'withdraw',
  'decline',
  'reject',
  'accept',
  'approve',
];

function confirmation(action: Action, request: RecruitmentRequest) {
  const student = request.student.displayName;
  switch (action) {
    case 'cancel':
      return {
        title: 'Cancel this invitation?',
        body: `${student} will be told. You can invite them again later.`,
        danger: true,
      };
    case 'withdraw':
      return {
        title: 'Withdraw your application?',
        body: 'The lead will be told. You can apply again right away if a place is still open.',
        danger: true,
      };
    case 'decline':
      return {
        title: 'Decline this invitation?',
        body: "The lead can't invite you again until you apply to this team yourself.",
        danger: true,
      };
    case 'reject':
      return {
        title: `Reject ${student}?`,
        body: "They can't apply to this team again unless you invite them.",
        danger: true,
      };
    default:
      return null;
  }
}

/** The actions this viewer could take on this kind of request while it's open. */
function actionsForRole(request: RecruitmentRequest): Action[] {
  const { isStudent, isLead } = request.viewer;
  if (request.type === 'application') {
    if (isStudent) return ['withdraw'];
    return isLead ? ['reject', 'approve'] : [];
  }
  if (isStudent) return ['decline', 'accept'];
  return isLead ? ['cancel'] : [];
}

export function RequestActions({
  request,
  size = 'sm',
  showExpired = false,
  openTeamOnJoin = false,
}: {
  request: RecruitmentRequest;
  size?: 'sm' | 'md';
  /** Keeps the expired request's actions on screen, disabled. */
  showExpired?: boolean;
  /** Accepting goes straight to the team page instead of staying here. */
  openTeamOnJoin?: boolean;
}) {
  const router = useRouter();
  const form = useApiForm();
  const now = useNow();
  const [confirming, setConfirming] = useState<Action | null>(null);

  const pastDue =
    request.status === 'pending' &&
    now !== null &&
    now >= new Date(request.expiresAt).getTime();
  const expired = pastDue || (showExpired && request.status === 'expired');

  const allowed = ORDER.filter((action) => request.viewer[PERMISSIONS[action]]);
  const shown = expired ? actionsForRole(request) : allowed;

  async function perform(action: Action) {
    const result = await form.run(() =>
      clientApi.post<RecruitmentRequest>(`/requests/${request.id}/${action}`),
    );
    setConfirming(null);
    if (result.ok) {
      if (action === 'accept' && openTeamOnJoin) {
        router.push(`/projects/${request.project.id}`);
      } else {
        router.refresh();
      }
      return;
    }
    // The loser of a last-place race, or a request that changed since the page
    // loaded. The refresh may remove this row, so the reason goes in the notice.
    if (result.error?.status === 409) {
      showNotice(result.error.message);
      form.clear();
      router.refresh();
    }
  }

  const dialog = confirming ? confirmation(confirming, request) : null;

  if (shown.length === 0 && !form.message) return null;

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        {shown.map((action) => {
          const main = action === 'accept' || action === 'approve';
          return (
            <Button
              key={action}
              size={size}
              variant={main ? 'primary' : 'secondary'}
              disabled={expired || form.busy}
              loading={form.busy && !dialog && main}
              onClick={() => {
                form.clear();
                if (confirmation(action, request)) setConfirming(action);
                else void perform(action);
              }}
            >
              {LABELS[action]}
            </Button>
          );
        })}
      </div>
      {expired && shown.length > 0 ? (
        <p className="text-sm text-ink-muted">
          This request expired, so it can&apos;t be answered any more.
        </p>
      ) : null}
      {!dialog ? <FormAlert form={form} /> : null}
      <ConfirmDialog
        open={dialog !== null}
        onClose={() => setConfirming(null)}
        onConfirm={() => {
          if (confirming) void perform(confirming);
        }}
        title={dialog?.title ?? ''}
        confirmLabel={confirming ? LABELS[confirming] : ''}
        danger={dialog?.danger}
        form={form}
      >
        {dialog?.body}
      </ConfirmDialog>
    </div>
  );
}
