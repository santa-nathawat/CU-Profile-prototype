'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Button, ConfirmDialog, FormAlert, Section } from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import type { Member, ProjectDetail } from '@/lib/types';
import { projectEnded } from '../project-rules';
import { useTeamAction } from './use-team-action';

export function LeaveTeam({ project }: { project: ProjectDetail }) {
  const router = useRouter();
  const form = useTeamAction();
  const [open, setOpen] = useState(false);
  if (
    !project.viewer.isMember ||
    project.viewer.isLead ||
    projectEnded(project.status)
  )
    return null;
  async function leave() {
    const result = await form.run(() =>
      clientApi.post(`/projects/${project.id}/leave`),
    );
    if (result.ok) {
      setOpen(false);
      router.refresh();
    }
  }
  return (
    <>
      <Button
        variant="ghost"
        onClick={() => {
          form.clear();
          setOpen(true);
        }}
      >
        Leave team
      </Button>
      <ConfirmDialog
        open={open}
        onClose={() => setOpen(false)}
        onConfirm={leave}
        title="Leave team?"
        confirmLabel="Leave team"
        danger
        form={form}
      >
        <p>
          Your role place becomes free and the lead is notified. You lose access
          to private team links. Your participation stays in your history.
        </p>
      </ConfirmDialog>
    </>
  );
}

export function MemberActions({
  project,
  member,
}: {
  project: ProjectDetail;
  member: Member;
}) {
  const router = useRouter();
  const form = useTeamAction();
  const [action, setAction] = useState<'remove' | 'transfer' | null>(null);
  if (!project.viewer.isLead || member.isLead || projectEnded(project.status))
    return null;
  async function confirm() {
    const result = await form.run(() =>
      action === 'remove'
        ? clientApi.delete(`/projects/${project.id}/members/${member.userId}`)
        : clientApi.post(`/projects/${project.id}/transfers`, {
            toUserId: member.userId,
          }),
    );
    if (result.ok) {
      setAction(null);
      router.refresh();
    }
  }
  function open(next: 'remove' | 'transfer') {
    form.clear();
    setAction(next);
  }
  return (
    <>
      <div className="flex flex-wrap gap-1">
        <Button
          size="sm"
          variant="secondary"
          disabled={Boolean(project.pendingTransfer)}
          onClick={() => open('transfer')}
        >
          Make lead
        </Button>
        <Button size="sm" variant="ghost" onClick={() => open('remove')}>
          Remove
        </Button>
      </div>
      <ConfirmDialog
        open={action !== null}
        onClose={() => setAction(null)}
        onConfirm={confirm}
        title={
          action === 'remove'
            ? `Remove ${member.displayName}?`
            : `Make ${member.displayName} lead?`
        }
        confirmLabel={
          action === 'remove' ? 'Remove member' : 'Request transfer'
        }
        danger={action === 'remove'}
        form={form}
      >
        <p>
          {action === 'remove'
            ? 'Their place becomes free and they are notified. Private team links disappear for them immediately. This does not remove them from the external LINE or Discord group.'
            : 'They must accept before leadership changes. You stay the lead until then. After acceptance, you stay a member and can leave the team.'}
        </p>
      </ConfirmDialog>
    </>
  );
}

export function Transfer({ project }: { project: ProjectDetail }) {
  const router = useRouter();
  const form = useTeamAction();
  const [accepting, setAccepting] = useState(false);
  const transfer = project.pendingTransfer;
  if (!transfer || projectEnded(project.status)) return null;
  async function decide(action: 'accept' | 'decline' | 'cancel') {
    const result = await form.run(() =>
      clientApi.post(`/transfers/${transfer!.id}/${action}`),
    );
    if (result.ok) {
      setAccepting(false);
      router.refresh();
    }
  }
  return (
    <Section title="Leadership transfer">
      <p className="mb-3 text-sm text-ink-muted">
        {project.viewer.isLead
          ? `Waiting for ${transfer.toDisplayName} to accept. You remain the lead until they do.`
          : 'The lead asked you to take over this team. Accepting makes you responsible for managing the project.'}
      </p>
      <div className="flex flex-wrap gap-2">
        {project.viewer.isLead ? (
          <Button
            variant="secondary"
            loading={form.busy}
            onClick={() => decide('cancel')}
          >
            Cancel transfer
          </Button>
        ) : (
          <>
            <Button
              disabled={form.busy}
              onClick={() => {
                form.clear();
                setAccepting(true);
              }}
            >
              Accept leadership
            </Button>
            <Button
              variant="secondary"
              loading={form.busy}
              onClick={() => decide('decline')}
            >
              Decline transfer
            </Button>
          </>
        )}
      </div>
      {!accepting ? <FormAlert form={form} className="mt-3" /> : null}
      <ConfirmDialog
        open={accepting}
        onClose={() => setAccepting(false)}
        onConfirm={() => decide('accept')}
        title="Accept leadership?"
        confirmLabel="Accept leadership"
        form={form}
      >
        <p>
          You become the team lead. The previous lead stays a member and can
          leave. There will still be exactly one lead.
        </p>
      </ConfirmDialog>
    </Section>
  );
}
