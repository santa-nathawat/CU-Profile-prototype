'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { ConfirmDialog, FormAlert, Switch } from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import { plural } from '@/lib/format';
import type { ProjectDetail } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';
import { projectEnded } from '../project-rules';

export function RecruitmentToggle({
  project,
  pendingCount,
}: {
  project: ProjectDetail;
  pendingCount: number;
}) {
  const router = useRouter();
  const form = useApiForm();
  const [confirming, setConfirming] = useState(false);

  const open = project.recruitmentStatus === 'open';
  const ended = projectEnded(project.status);

  async function change() {
    const result = await form.run(() =>
      clientApi.post(`/projects/${project.id}/recruitment`, {
        status: open ? 'closed' : 'open',
      }),
    );
    if (result.ok) {
      setConfirming(false);
      router.refresh();
    }
  }

  return (
    <>
      <Switch
        label={open ? 'Recruitment is open' : 'Recruitment is closed'}
        hint={
          ended
            ? "This project has ended, so recruitment can't reopen."
            : open
              ? 'Students can apply and you can invite people.'
              : 'Students cannot apply. It stays closed, even if someone leaves, until you reopen it.'
        }
        checked={open}
        disabled={ended}
        onChange={() => {
          form.clear();
          setConfirming(true);
        }}
      />
      {!confirming ? <FormAlert form={form} className="mt-3" /> : null}
      <ConfirmDialog
        open={confirming}
        onClose={() => setConfirming(false)}
        onConfirm={change}
        title={open ? 'Close recruitment?' : 'Reopen recruitment?'}
        confirmLabel={open ? 'Close recruitment' : 'Reopen recruitment'}
        danger={open}
        form={form}
      >
        {open ? (
          <p>
            Closing cancels all pending requests for this team (
            {plural(pendingCount, 'request')} right now) and tells those
            students. Reopening later restores none of them.
          </p>
        ) : (
          <p>
            Students can apply again. Requests cancelled when you closed
            recruitment stay cancelled.
          </p>
        )}
      </ConfirmDialog>
    </>
  );
}
