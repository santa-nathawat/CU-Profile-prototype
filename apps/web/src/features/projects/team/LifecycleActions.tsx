'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Alert,
  Button,
  ConfirmDialog,
  Field,
  FormAlert,
  Input,
  Modal,
  Section,
} from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import { formatDate, plural } from '@/lib/format';
import type { ProjectDetail } from '@/lib/types';
import { useTeamAction } from './use-team-action';

type Action = 'start' | 'extend' | 'complete' | 'cancel' | 'delete';

function bangkokDate() {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Bangkok',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date());
}

function DateAction({
  project,
  action,
  pendingCount,
  onClose,
}: {
  project: ProjectDetail;
  action: 'start' | 'extend';
  pendingCount: number;
  onClose: () => void;
}) {
  const router = useRouter();
  const form = useTeamAction();
  const [date, setDate] = useState('');
  const [closed, setClosed] = useState(project.recruitmentStatus === 'closed');
  const starting = action === 'start';

  async function closeRecruitment() {
    const result = await form.run(() =>
      clientApi.post(`/projects/${project.id}/recruitment`, {
        status: 'closed',
      }),
    );
    if (result.ok) {
      setClosed(true);
      router.refresh();
    }
  }

  async function save(event: React.FormEvent) {
    event.preventDefault();
    if (
      !date ||
      (starting
        ? date < bangkokDate()
        : date <= (project.expectedEndDate ?? ''))
    ) {
      form.fail({
        expectedEndDate: starting
          ? 'Choose today or a later date.'
          : 'Choose a date later than the current end date.',
      });
      return;
    }
    const result = await form.run(() =>
      clientApi.post(`/projects/${project.id}/${action}`, {
        expectedEndDate: date,
      }),
    );
    if (result.ok) {
      onClose();
      router.refresh();
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      busy={form.busy}
      title={starting ? 'Start project' : 'Extend end date'}
    >
      <form onSubmit={save} noValidate className="space-y-4">
        {starting ? (
          <div className="space-y-3">
            <h3 className="font-medium">1. Close recruitment</h3>
            {closed ? (
              <Alert tone="success">Recruitment is closed.</Alert>
            ) : (
              <>
                <p className="text-sm text-ink-muted">
                  Closing cancels {plural(pendingCount, 'pending request')} and
                  notifies those students. Reopening later will not restore
                  them.
                </p>
                <Button
                  variant="secondary"
                  loading={form.busy}
                  onClick={closeRecruitment}
                >
                  Close recruitment
                </Button>
              </>
            )}
            <p className="text-sm text-ink-muted">
              Open roles can stay unfilled. Closing recruitment takes effect
              immediately, even if you leave this dialog.
            </p>
          </div>
        ) : (
          <p className="text-sm text-ink-muted">
            Current expected end:{' '}
            {project.expectedEndDate
              ? formatDate(project.expectedEndDate)
              : 'Not set'}
            . Extending clears the end date reminder.
          </p>
        )}
        <Field
          label={starting ? '2. Expected end date' : 'New expected end date'}
          hint="Bangkok date"
          required
          error={form.fields.expectedEndDate}
        >
          <Input
            type="date"
            value={date}
            min={
              starting ? bangkokDate() : (project.expectedEndDate ?? undefined)
            }
            onChange={(event) => setDate(event.target.value)}
            disabled={form.busy}
          />
        </Field>
        <FormAlert form={form} />
        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="secondary" onClick={onClose} disabled={form.busy}>
            Go back
          </Button>
          <Button
            type="submit"
            loading={form.busy}
            disabled={(starting && !closed) || !date}
          >
            {starting ? 'Start project' : 'Extend end date'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

export function LifecycleActions({
  project,
  pendingCount = 0,
}: {
  project: ProjectDetail;
  pendingCount?: number;
}) {
  const router = useRouter();
  const form = useTeamAction();
  const [action, setAction] = useState<Action | null>(null);
  if (
    !project.viewer.isLead ||
    project.status === 'completed' ||
    project.status === 'cancelled'
  )
    return null;
  const unstarted = project.status === 'not_started';
  const title =
    action === 'complete'
      ? 'Complete project?'
      : action === 'cancel'
        ? 'Cancel project?'
        : 'Delete team?';
  function open(next: Action) {
    form.clear();
    setAction(next);
  }
  async function confirm() {
    const result = await form.run(() =>
      action === 'delete'
        ? clientApi.delete(`/projects/${project.id}`)
        : clientApi.post(`/projects/${project.id}/${action}`),
    );
    if (result.ok) {
      setAction(null);
      if (action === 'delete') router.replace('/my-teams');
      router.refresh();
    }
  }
  return (
    <Section title="Project actions">
      {project.endDateReached ? (
        <Alert tone="warning" className="mb-4">
          The expected end date has arrived. Complete the project or extend its
          end date. The project stays in progress until you decide.
        </Alert>
      ) : null}
      {project.expectedEndDate ? (
        <p className="mb-3 text-sm text-ink-muted">
          Expected end: {formatDate(project.expectedEndDate)}
        </p>
      ) : null}
      <div className="flex flex-wrap gap-2">
        {unstarted ? (
          <>
            <Button onClick={() => open('start')}>Start project</Button>
            <Button variant="ghost" onClick={() => open('delete')}>
              Delete team
            </Button>
          </>
        ) : (
          <>
            <Button onClick={() => open('complete')}>Complete project</Button>
            <Button variant="secondary" onClick={() => open('extend')}>
              Extend end date
            </Button>
            <Button variant="ghost" onClick={() => open('cancel')}>
              Cancel project
            </Button>
          </>
        )}
      </div>
      {action === 'start' || action === 'extend' ? (
        <DateAction
          key={action}
          project={project}
          action={action}
          pendingCount={pendingCount}
          onClose={() => setAction(null)}
        />
      ) : null}
      <ConfirmDialog
        open={
          action === 'complete' || action === 'cancel' || action === 'delete'
        }
        onClose={() => setAction(null)}
        onConfirm={confirm}
        title={title}
        confirmLabel={
          action === 'complete'
            ? 'Complete project'
            : action === 'cancel'
              ? 'Cancel project'
              : 'Delete team'
        }
        danger
        form={form}
      >
        {action === 'complete' ? (
          <ul className="list-disc space-y-2 pl-5">
            <li>Membership and participation history will be frozen.</li>
            <li>
              The {plural(project.members.length, 'current member')} will be
              fixed as eligible participants. They can rate each other for 14
              days. No one can rate themselves.
            </li>
            <li>
              Recruitment closes and all pending applications and invitations
              are cancelled.
            </li>
            <li>Completion cannot be reversed in this prototype.</li>
          </ul>
        ) : action === 'cancel' ? (
          <p>
            Cancellation preserves participation history and freezes membership.
            Recruitment closes and all pending applications and invitations are
            cancelled. Ratings will not open. This cannot be undone.
          </p>
        ) : (
          <p>
            This removes the unstarted team from search, ends every membership
            and cancels all pending applications and invitations. Those students
            will be notified. This cannot be undone.
          </p>
        )}
      </ConfirmDialog>
    </Section>
  );
}
