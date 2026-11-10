'use client';

import { useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Button,
  Field,
  FormAlert,
  Input,
  Section,
  Select,
} from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import { plural } from '@/lib/format';
import type { Category, NewProjectInput, ProjectDetail } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';
import {
  detailsErrors,
  detailsInput,
  rolesErrors,
  totalPlaces,
  type ProjectValues,
  type RoleDraft,
} from './project-form';
import { ProjectDetailsFields } from './ProjectDetailsFields';

const EMPTY: ProjectValues = {
  name: '',
  description: '',
  categoryId: '',
  durationValue: '',
  durationUnit: 'month',
};

export function NewProjectForm({ categories }: { categories: Category[] }) {
  const router = useRouter();
  const form = useApiForm();
  const formElement = useRef<HTMLFormElement>(null);
  const nextKey = useRef(1);

  const [values, setValues] = useState(EMPTY);
  const [roles, setRoles] = useState<RoleDraft[]>([
    { key: 0, name: '', capacity: '2' },
  ]);
  const [leadRoleKey, setLeadRoleKey] = useState(0);

  // Removing the role you picked falls back to the first one.
  const leadIndex = Math.max(
    0,
    roles.findIndex((role) => role.key === leadRoleKey),
  );
  const places = totalPlaces(roles);

  function updateRole(key: number, patch: Partial<RoleDraft>) {
    setRoles((current) =>
      current.map((role) => (role.key === key ? { ...role, ...patch } : role)),
    );
  }

  function addRole() {
    const key = nextKey.current++;
    setRoles((current) => [...current, { key, name: '', capacity: '1' }]);
  }

  function focusFirstError() {
    requestAnimationFrame(() => {
      formElement.current
        ?.querySelector<HTMLElement>('[aria-invalid="true"], [data-form-error]')
        ?.focus();
    });
  }

  async function publish(event: React.FormEvent) {
    event.preventDefault();
    const errors = { ...detailsErrors(values), ...rolesErrors(roles) };
    if (Object.keys(errors).length > 0) {
      form.fail(errors);
      focusFirstError();
      return;
    }
    const input: NewProjectInput = {
      ...detailsInput(values),
      roles: roles.map((role) => ({
        name: role.name.trim(),
        capacity: Number(role.capacity),
      })),
      leadRoleIndex: leadIndex,
    };
    const result = await form.run(() =>
      clientApi.post<ProjectDetail>('/projects', input),
    );
    // Replace, so Back from the team page can't return to a form already sent.
    if (result.ok) router.replace(`/projects/${result.data.id}`);
    else focusFirstError();
  }

  return (
    <form ref={formElement} onSubmit={publish} noValidate className="space-y-4">
      <Section title="Project details">
        <ProjectDetailsFields
          values={values}
          onChange={(patch) =>
            setValues((current) => ({ ...current, ...patch }))
          }
          errors={form.fields}
          categories={categories}
        />
      </Section>

      <Section
        title="Roles"
        description="The places you want to fill. Your own place counts as one of them."
      >
        <ul className="space-y-4">
          {roles.map((role, index) => (
            <li
              key={role.key}
              className="grid items-start gap-3 sm:grid-cols-[1fr_7rem_auto]"
            >
              <Field
                label={`Role ${index + 1} name`}
                required
                error={form.fields[`roles.${role.key}.name`]}
              >
                <Input
                  value={role.name}
                  maxLength={60}
                  placeholder="Backend developer"
                  onChange={(event) =>
                    updateRole(role.key, { name: event.target.value })
                  }
                />
              </Field>
              <Field
                label="Places"
                required
                error={form.fields[`roles.${role.key}.capacity`]}
              >
                <Input
                  type="number"
                  inputMode="numeric"
                  min={1}
                  step={1}
                  value={role.capacity}
                  onChange={(event) =>
                    updateRole(role.key, { capacity: event.target.value })
                  }
                />
              </Field>
              {roles.length > 1 ? (
                <Button
                  variant="ghost"
                  className="sm:mt-7"
                  aria-label={`Remove role ${index + 1}`}
                  onClick={() =>
                    setRoles((current) =>
                      current.filter((item) => item.key !== role.key),
                    )
                  }
                >
                  Remove
                </Button>
              ) : null}
            </li>
          ))}
        </ul>
        <Button variant="secondary" className="mt-4" onClick={addRole}>
          Add a role
        </Button>
        {form.fields.roles ? (
          <p
            data-form-error
            tabIndex={-1}
            role="alert"
            className="mt-3 text-sm text-danger"
          >
            {form.fields.roles}
          </p>
        ) : null}
      </Section>

      <Section title="Your place in the team">
        <Field
          label="Your role"
          required
          error={form.fields.leadRoleIndex}
          hint={
            places >= 2
              ? `You lead the team and take one place. ${plural(places - 1, 'place')} stay open for others.`
              : 'You lead the team and take one place. At least one more must stay open for others.'
          }
        >
          <Select
            value={roles[leadIndex]?.key}
            onChange={(event) => setLeadRoleKey(Number(event.target.value))}
          >
            {roles.map((role, index) => (
              <option key={role.key} value={role.key}>
                {role.name.trim() || `Role ${index + 1}`}
              </option>
            ))}
          </Select>
        </Field>
      </Section>

      <FormAlert form={form} />
      <Button type="submit" loading={form.busy}>
        Publish project
      </Button>
    </form>
  );
}
