import 'server-only';
import { cache } from 'react';
import { cookies } from 'next/headers';
import { redirect } from 'next/navigation';
import { apiRequest } from './api';
import { ApiError } from './api-error';
import type { Me } from './types';

/** The signed-in student, or null. Other API failures still throw. */
export const getMe = cache(async (): Promise<Me | null> => {
  if (!(await cookies()).has('cu_session')) return null;
  try {
    return await apiRequest<Me>('/me');
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) return null;
    throw error;
  }
});

/** For public pages, which must keep working when the API is down or asleep. */
export async function getMeIfReachable(): Promise<Me | null> {
  if (!(await cookies()).has('cu_session')) return null;
  try {
    return await apiRequest<Me>('/me', { timeoutMs: 3000 });
  } catch {
    return null;
  }
}

/** Signed out goes to sign-in. An unfinished profile goes to onboarding. */
export async function requireStudent(): Promise<Me> {
  const me = await getMe();
  if (!me) redirect('/sign-in');
  if (!me.profileComplete) redirect('/onboarding');
  return me;
}
