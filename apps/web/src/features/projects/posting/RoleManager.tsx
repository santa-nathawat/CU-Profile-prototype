'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import {
  Badge,
  Button,
  ConfirmDialog,
  Field,
  FormAlert,
  Input,
  Stepper,
} from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import { plural } from '@/lib/format';
import type { Role } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';
import { placesLeft } from '../project-rules';

function RoleRow({
  projectId,
  role,
  pendingCount,
}: {
  projectId: string;
  role: Role;
  pendingCount: number;
}) {
  const router = useRouter();
  const form = useApiForm();
  const [refreshing, startRefresh] = useTransition();
  const [removing, setRemoving] = useState(false);
  const path = `/projects/${projectId}/roles/${role.id}`;

  async function setCapacity(capacity: number) {
    const result = await form.run(() => clientApi.patch(path, { capacity }));
    if (result.ok) startRefresh(() => router.refresh());
  }

  async function remove() {
    const result = await form.run(() => clientApi.delete(path));
    if (result.ok) {
      setRemoving(false);
      router.refresh();
    }
  }

  return (
    <li className="py-3 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-center gap-3">
        <div className="min-w-0 flex-1">
          <p className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{role.name}</span>
            <Badge tone={role.available > 0 ? 'success' : 'neutral'}>
              {placesLeft(role)}
            </Badge>
          </p>
          <p className="text-sm text-ink-muted">
            {plural(role.taken, 'member')}, {plural(role.capacity, 'place')}
          </p>
        </div>
        <Stepper
          value={role.capacity}
          label={`places in ${role.name}`}
          disabled={form.busy || refreshing}
          onChange={setCapacity}
        />
        <Button
          variant="ghost"
          size="sm"
          aria-label={`Remove role ${role.name}`}
          onClick={() => {
            form.clear();
            setRemoving(true);
          }}
        >
          Remove
        </Button>
      </div>
      {!removing ? <FormAlert form={form} className="mt-2" /> : null}
      <ConfirmDialog
        open={removing}
        onClose={() => setRemoving(false)}
        onConfirm={remove}
        title={`Remove ${role.name}?`}
        confirmLabel="Remove role"
        danger
        form={form}
      >
        {pendingCount > 0 ? (
          <p>
            This cancels {plural(pendingCount, 'pending request')} for this role
            and tells those students the role is no longer available.
          </p>
        ) : (
          <p>No pending requests are waiting on this role.</p>
        )}
      </ConfirmDialog>
    </li>
  );
}

function AddRole({ projectId }: { projectId: string }) {
  const router = useRouter();
  const form = useApiForm();
  const [name, setName] = useState('');
  const [capacity, setCapacity] = useState('1');

  async function add(event: React.FormEvent) {
    event.preventDefault();
    if (!name.trim()) {
      form.fail({ name: 'Give the role a name.' });
      return;
    }
    const places = Number(capacity);
    if (!Number.isInteger(places) || places < 1) {
      form.fail({ capacity: 'Enter 1 or more.' });
      return;
    }
    const result = await form.run(() =>
      clientApi.post<Role>(`/projects/${projectId}/roles`, {
        name: name.trim(),
        capacity: places,
      }),
    );
    if (result.ok) {
      setName('');
      setCapacity('1');
      router.refresh();
    }
  }

  return (
    <form onSubmit={add} noValidate className="mt-4 border-t border-line pt-4">
      <div className="grid items-start gap-3 sm:grid-cols-[1fr_7rem_auto]">
        <Field label="New role" error={form.fields.name}>
          <Input
            value={name}
            maxLength={60}
            placeholder="Designer"
            onChange={(event) => setName(event.target.value)}
          />
        </Field>
        <Field label="Places" error={form.fields.capacity}>
          <Input
            type="number"
            inputMode="numeric"
            min={1}
            step={1}
            value={capacity}
            onChange={(event) => setCapacity(event.target.value)}
          />
        </Field>
        <Button
          type="submit"
          variant="secondary"
          loading={form.busy}
          className="sm:mt-7"
        >
          Add role
        </Button>
      </div>
      <FormAlert form={form} className="mt-3" />
    </form>
  );
}

export function RoleManager({
  projectId,
  roles,
  pendingByRole,
}: {
  projectId: string;
  roles: Role[];
  pendingByRole: Record<string, number>;
}) {
  return (
    <>
      <ul className="divide-y divide-line">
        {roles.map((role) => (
          <RoleRow
            key={role.id}
            projectId={projectId}
            role={role}
            pendingCount={pendingByRole[role.id] ?? 0}
          />
        ))}
      </ul>
      <AddRole projectId={projectId} />
    </>
  );
}
