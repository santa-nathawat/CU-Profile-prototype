'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Alert, Button, FormAlert } from '@/components/ui';
import {
  profileInput,
  profileValues,
  requiredErrors,
  type ProfileValues,
} from '@/features/profiles/profile-form';
import {
  BasicFields,
  WorkingStyleField,
} from '@/features/profiles/ProfileFields';
import { clientApi } from '@/lib/api-client';
import type { Faculty, MyProfile } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';

export function ProfileForm({
  profile,
  faculties,
}: {
  profile: MyProfile;
  faculties: Faculty[];
}) {
  const router = useRouter();
  const form = useApiForm();
  const [values, setValues] = useState<ProfileValues>(profileValues(profile));
  const [saved, setSaved] = useState(false);

  function change(patch: Partial<ProfileValues>) {
    setSaved(false);
    setValues((current) => ({ ...current, ...patch }));
  }

  async function save(event: React.FormEvent) {
    event.preventDefault();
    setSaved(false);
    // Removing any of these would hide the profile and send the student back
    // to onboarding, so they stay required once the profile exists.
    const errors = requiredErrors(values);
    if (Object.keys(errors).length > 0) {
      form.fail(errors);
      return;
    }
    const result = await form.run(() =>
      clientApi.put<MyProfile>('/me/profile', profileInput(values)),
    );
    if (result.ok) {
      setSaved(true);
      router.refresh();
    }
  }

  return (
    <form onSubmit={save} noValidate className="space-y-5">
      <BasicFields
        values={values}
        onChange={change}
        errors={form.fields}
        faculties={faculties}
      />
      <WorkingStyleField
        value={values.workingStyle}
        onChange={(workingStyle) => change({ workingStyle })}
        error={form.fields.workingStyle}
      />
      <FormAlert form={form} />
      {saved ? <Alert tone="success">Your profile was saved.</Alert> : null}
      <Button type="submit" loading={form.busy}>
        Save changes
      </Button>
    </form>
  );
}
