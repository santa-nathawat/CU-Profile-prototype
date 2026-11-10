'use client';

import { useState } from 'react';
import { Button, Card, FormAlert } from '@/components/ui';
import { AvailabilityGrid } from '@/features/profiles/AvailabilityGrid';
import { CvManager } from '@/features/profiles/CvManager';
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
import { reloadTo } from '@/lib/nav';
import type { AvailabilitySlot, Faculty, MyProfile } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';

export function OnboardingWizard({
  userId,
  profile,
  faculties,
}: {
  userId: string;
  profile: MyProfile | null;
  faculties: Faculty[];
}) {
  const form = useApiForm();
  const [step, setStep] = useState<1 | 2>(1);
  const [values, setValues] = useState<ProfileValues>(profileValues(profile));
  const [slots, setSlots] = useState<AvailabilitySlot[]>(
    profile?.availability ?? [],
  );

  const change = (patch: Partial<ProfileValues>) =>
    setValues((current) => ({ ...current, ...patch }));

  async function saveBasics(event: React.FormEvent) {
    event.preventDefault();
    const errors = requiredErrors(values);
    if (Object.keys(errors).length > 0) {
      form.fail(errors);
      return;
    }
    const result = await form.run(() =>
      clientApi.put<MyProfile>('/me/profile', profileInput(values)),
    );
    if (result.ok) {
      setStep(2);
      window.scrollTo(0, 0);
    }
  }

  async function finish(event: React.FormEvent) {
    event.preventDefault();
    const result = await form.run(async () => {
      await clientApi.put('/me/profile', profileInput(values));
      await clientApi.put('/me/availability', { slots });
    });
    // Full load, so the app shell reads the finished profile.
    if (result.ok) reloadTo('/projects');
  }

  return (
    <div className="mx-auto w-full max-w-2xl">
      <p className="text-sm font-medium text-accent-ink">Step {step} of 2</p>
      {step === 1 ? (
        <>
          <h1 className="mt-1 text-2xl font-semibold">Set up your profile</h1>
          <p className="mt-1 text-ink-muted">
            A name, a faculty or school and one skill are enough for people to
            find you.
          </p>
          <Card className="mt-6">
            <form onSubmit={saveBasics} noValidate className="space-y-5">
              <BasicFields
                values={values}
                onChange={change}
                errors={form.fields}
                faculties={faculties}
              />
              <FormAlert form={form} />
              <div className="flex justify-end">
                <Button type="submit" loading={form.busy}>
                  Next
                </Button>
              </div>
            </form>
          </Card>
        </>
      ) : (
        <>
          <h1 className="mt-1 text-2xl font-semibold">A bit more about you</h1>
          <p className="mt-1 text-ink-muted">
            All optional. Anything you skip shows as Not specified, and you can
            add it later in Settings.
          </p>
          <form onSubmit={finish} className="mt-6 space-y-6">
            <Card>
              <h2 className="mb-3 text-lg font-semibold">CV</h2>
              <CvManager
                userId={userId}
                fileName={profile?.cv.fileName ?? null}
                shared={profile?.cvShared ?? false}
              />
            </Card>
            <Card>
              <h2 className="mb-3 text-lg font-semibold">
                Weekly availability
              </h2>
              <AvailabilityGrid value={slots} onChange={setSlots} />
            </Card>
            <Card>
              <WorkingStyleField
                value={values.workingStyle}
                onChange={(workingStyle) => change({ workingStyle })}
                error={form.fields.workingStyle}
              />
            </Card>
            <FormAlert form={form} />
            <div className="flex justify-between gap-2">
              <Button
                variant="secondary"
                disabled={form.busy}
                onClick={() => {
                  form.clear();
                  setStep(1);
                }}
              >
                Back
              </Button>
              <Button type="submit" loading={form.busy}>
                Save profile
              </Button>
            </div>
          </form>
        </>
      )}
    </div>
  );
}
