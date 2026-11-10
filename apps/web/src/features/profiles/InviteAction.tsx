'use client';

import Link from 'next/link';
import { useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Alert,
  Button,
  Field,
  FormAlert,
  Modal,
  Select,
} from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import type { InvitableProject, RecruitmentRequest } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';

export function InviteAction({
  studentId,
  studentName,
  projects,
  size = 'md',
  label = 'Invite to my team',
}: {
  studentId: string;
  studentName: string;
  projects: InvitableProject[];
  size?: 'sm' | 'md';
  label?: string;
}) {
  const router = useRouter();
  const form = useApiForm();
  const [open, setOpen] = useState(false);
  const [sent, setSent] = useState(false);
  const [projectId, setProjectId] = useState(projects[0]?.id ?? '');
  const [roleId, setRoleId] = useState('');

  const project = projects.find((item) => item.id === projectId);
  const openRoles = project?.roles.filter((role) => role.available > 0) ?? [];
  // Keeps a stale choice from surviving a change of project.
  const chosenRole =
    openRoles.find((role) => role.id === roleId) ?? openRoles[0] ?? null;

  function show() {
    form.clear();
    setSent(false);
    setOpen(true);
  }

  async function send(event: React.FormEvent) {
    event.preventDefault();
    if (!project || !chosenRole) return;
    const result = await form.run(() =>
      clientApi.post<RecruitmentRequest>(
        `/projects/${project.id}/invitations`,
        {
          roleId: chosenRole.id,
          studentId,
        },
      ),
    );
    if (result.ok) setSent(true);
    // A 409 means the role or team changed, so the choices here may be stale.
    if (result.ok || result.error?.status === 409) router.refresh();
  }

  return (
    <>
      <Button size={size} onClick={show}>
        {label}
      </Button>
      <Modal
        open={open}
        onClose={() => setOpen(false)}
        busy={form.busy}
        title={sent ? 'Invitation sent' : `Invite ${studentName}`}
      >
        {sent ? (
          <div className="space-y-4">
            <Alert tone="success">
              {studentName} has 48 hours to accept or decline. You can track it
              in{' '}
              <Link href="/requests" className="underline underline-offset-2">
                Requests
              </Link>
              .
            </Alert>
            <div className="flex justify-end">
              <Button
                variant="secondary"
                autoFocus
                onClick={() => setOpen(false)}
              >
                Done
              </Button>
            </div>
          </div>
        ) : (
          <form onSubmit={send} className="space-y-4">
            <Field label="Project" error={form.fields.projectId}>
              <Select
                data-autofocus
                value={projectId}
                onChange={(event) => {
                  setProjectId(event.target.value);
                  form.clear();
                }}
              >
                {projects.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Role" error={form.fields.roleId}>
              <Select
                value={chosenRole?.id ?? ''}
                disabled={openRoles.length === 0}
                onChange={(event) => {
                  setRoleId(event.target.value);
                  form.clear();
                }}
              >
                {project?.roles.map((role) => (
                  <option
                    key={role.id}
                    value={role.id}
                    disabled={role.available === 0}
                  >
                    {role.available === 0
                      ? `${role.name} (full)`
                      : `${role.name} (${role.available} open)`}
                  </option>
                ))}
              </Select>
            </Field>
            {openRoles.length === 0 ? (
              <Alert tone="warning">
                Every role in this project is full, so you can&apos;t invite
                anyone to it right now. Add a role or free up a place first.
              </Alert>
            ) : (
              <p className="text-sm text-ink-muted">
                {studentName} will be invited to join{' '}
                <strong className="font-medium text-ink">
                  {project?.name}
                </strong>{' '}
                as{' '}
                <strong className="font-medium text-ink">
                  {chosenRole?.name}
                </strong>
                , and can accept or decline within 48 hours.
              </p>
            )}
            <FormAlert form={form} />
            <div className="flex flex-wrap justify-end gap-2">
              <Button
                variant="secondary"
                disabled={form.busy}
                onClick={() => setOpen(false)}
              >
                Cancel
              </Button>
              <Button type="submit" loading={form.busy} disabled={!chosenRole}>
                Send invitation
              </Button>
            </div>
          </form>
        )}
      </Modal>
    </>
  );
}
