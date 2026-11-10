'use client';

import Link from 'next/link';
import { useState } from 'react';
import { Button, FormAlert, Modal } from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import { reloadTo } from '@/lib/nav';
import { useApiForm } from '@/lib/use-api-form';

export function CloseAccount() {
  const form = useApiForm();
  const [open, setOpen] = useState(false);
  const [blocked, setBlocked] = useState(false);

  async function close() {
    const result = await form.run(() => clientApi.post('/me/close'));
    if (result.ok) {
      reloadTo('/');
    } else if (result.error?.code === 'must_transfer_or_delete') {
      setBlocked(true);
    }
  }

  return (
    <>
      <Button
        variant="secondary"
        onClick={() => {
          form.clear();
          setBlocked(false);
          setOpen(true);
        }}
      >
        Close account
      </Button>
      <Modal
        open={open}
        onClose={() => setOpen(false)}
        busy={form.busy}
        title="Close your account?"
        footer={
          <>
            <Button
              variant="secondary"
              disabled={form.busy}
              onClick={() => setOpen(false)}
              data-autofocus
            >
              Keep account
            </Button>
            {!blocked ? (
              <Button variant="danger" loading={form.busy} onClick={close}>
                Close account
              </Button>
            ) : null}
          </>
        }
      >
        <ul className="list-disc space-y-1 pl-5 text-ink-muted">
          <li>Your profile and ratings disappear right away.</li>
          <li>Your pending applications and invitations are cancelled.</li>
          <li>You leave any team that hasn&apos;t ended.</li>
        </ul>
        <p className="mt-3 text-sm text-ink-muted">
          If you lead a team that hasn&apos;t ended, transfer it or delete it
          first.
        </p>
        <FormAlert form={form} className="mt-4" />
        {blocked ? (
          <p className="mt-3 text-sm">
            <Link href="/my-teams" className="link">
              Go to My teams
            </Link>
          </p>
        ) : null}
      </Modal>
    </>
  );
}
