'use client';

import { useRouter } from 'next/navigation';
import { Button, FormAlert } from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import { useApiForm } from '@/lib/use-api-form';

export function MarkAllRead() {
  const router = useRouter();
  const form = useApiForm();

  async function markAll() {
    const result = await form.run(() =>
      clientApi.post('/notifications/read-all'),
    );
    if (result.ok) router.refresh();
  }

  return (
    <div className="flex flex-col items-end gap-2">
      <Button
        variant="secondary"
        size="sm"
        loading={form.busy}
        onClick={markAll}
      >
        Mark all read
      </Button>
      <FormAlert form={form} />
    </div>
  );
}
