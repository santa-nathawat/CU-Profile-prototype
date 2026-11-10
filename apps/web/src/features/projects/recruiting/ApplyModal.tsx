'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Button,
  Field,
  FormAlert,
  Modal,
  Select,
  Textarea,
} from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import type { ProjectCard, RecruitmentRequest } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';

function ApplyForm({
  project,
  initialRoleId,
  onClose,
  form,
}: {
  project: ProjectCard;
  initialRoleId: string;
  onClose: () => void;
  form: ReturnType<typeof useApiForm>;
}) {
  const router = useRouter();
  const [roleId, setRoleId] = useState(initialRoleId);
  const [message, setMessage] = useState('');

  const openRoles = project.roles.filter((role) => role.available > 0);
  // The role may have filled up since the page loaded and refreshed.
  const chosen =
    openRoles.find((role) => role.id === roleId) ?? openRoles[0] ?? null;

  async function send(event: React.FormEvent) {
    event.preventDefault();
    if (!chosen) return;
    const note = message.trim();
    const result = await form.run(() =>
      clientApi.post<RecruitmentRequest>(
        `/projects/${project.id}/applications`,
        { roleId: chosen.id, ...(note ? { message: note } : {}) },
      ),
    );
    if (result.ok) onClose();
    if (result.ok || result.error?.status === 409) router.refresh();
  }

  return (
    <form onSubmit={send} noValidate className="space-y-4">
      <Field label="Role" required error={form.fields.roleId}>
        <Select
          data-autofocus
          value={chosen?.id ?? ''}
          disabled={openRoles.length === 0}
          onChange={(event) => setRoleId(event.target.value)}
        >
          {openRoles.map((role) => (
            <option key={role.id} value={role.id}>
              {role.name} ({role.available} of {role.capacity} left)
            </option>
          ))}
        </Select>
      </Field>
      <Field
        label="Message to the lead"
        error={form.fields.message}
        hint="Optional. Say why you fit this role."
      >
        <Textarea
          value={message}
          maxLength={1000}
          onChange={(event) => setMessage(event.target.value)}
        />
      </Field>
      <p className="text-sm text-ink-muted">
        The lead has 48 hours to answer. You can withdraw before then.
      </p>
      <FormAlert form={form} />
      <div className="flex flex-wrap justify-end gap-2">
        <Button variant="secondary" disabled={form.busy} onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" loading={form.busy} disabled={!chosen}>
          Send application
        </Button>
      </div>
    </form>
  );
}

export function ApplyModal({
  project,
  roleId,
  onClose,
}: {
  project: ProjectCard;
  /** The role picked on the page. Null keeps the modal closed. */
  roleId: string | null;
  onClose: () => void;
}) {
  const form = useApiForm();
  return (
    <Modal
      open={roleId !== null}
      onClose={onClose}
      busy={form.busy}
      title={`Apply to ${project.name}`}
    >
      <ApplyForm
        project={project}
        initialRoleId={roleId ?? ''}
        onClose={onClose}
        form={form}
      />
    </Modal>
  );
}
