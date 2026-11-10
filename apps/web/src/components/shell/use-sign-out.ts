'use client';

import { useState } from 'react';
import { clientApi } from '@/lib/api-client';
import { reloadTo } from '@/lib/nav';

/** Full page load after sign-out, so no signed-in page stays in the client cache. */
export function useSignOut() {
  const [busy, setBusy] = useState(false);
  async function signOut() {
    setBusy(true);
    try {
      await clientApi.post('/auth/sign-out');
    } finally {
      reloadTo('/sign-in');
    }
  }
  return { busy, signOut };
}
