'use client';

import { useRouter } from 'next/navigation';
import { showNotice } from '@/lib/notice';
import { useApiForm } from '@/lib/use-api-form';

export function useTeamAction() {
  const router = useRouter();
  const form = useApiForm();
  async function run<T>(action: () => Promise<T>) {
    const result = await form.run(action);
    if (!result.ok && result.error?.status === 409) {
      showNotice(result.error.message);
      router.refresh();
    }
    return result;
  }
  return { ...form, run };
}
