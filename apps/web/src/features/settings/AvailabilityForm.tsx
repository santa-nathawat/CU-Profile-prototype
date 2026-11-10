'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Alert, Button, FormAlert } from '@/components/ui';
import { AvailabilityGrid } from '@/features/profiles/AvailabilityGrid';
import { clientApi } from '@/lib/api-client';
import type { AvailabilitySlot } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';

export function AvailabilityForm({ initial }: { initial: AvailabilitySlot[] }) {
  const router = useRouter();
  const form = useApiForm();
  const [slots, setSlots] = useState(initial);
  const [saved, setSaved] = useState(false);

  async function save() {
    setSaved(false);
    const result = await form.run(() =>
      clientApi.put('/me/availability', { slots }),
    );
    if (result.ok) {
      setSaved(true);
      router.refresh();
    }
  }

  return (
    <div className="space-y-4">
      <AvailabilityGrid
        value={slots}
        onChange={(next) => {
          setSaved(false);
          setSlots(next);
        }}
      />
      <FormAlert form={form} />
      {saved ? (
        <Alert tone="success">Your availability was saved.</Alert>
      ) : null}
      <Button loading={form.busy} onClick={save}>
        Save availability
      </Button>
    </div>
  );
}
