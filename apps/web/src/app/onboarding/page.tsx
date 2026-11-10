import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { OnboardingWizard } from '@/features/onboarding/OnboardingWizard';
import { apiGet } from '@/lib/api';
import { ApiError } from '@/lib/api-error';
import { getMe } from '@/lib/auth';
import type { Faculty, ItemList, MyProfile } from '@/lib/types';

export const metadata: Metadata = { title: 'Set up your profile' };

// A brand new account may have no profile row yet.
async function loadProfile(): Promise<MyProfile | null> {
  try {
    return await apiGet<MyProfile>('/me/profile');
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return null;
    throw error;
  }
}

export default async function OnboardingPage() {
  const me = await getMe();
  if (!me) redirect('/sign-in');
  if (me.profileComplete) redirect('/projects');

  const [profile, faculties] = await Promise.all([
    loadProfile(),
    apiGet<ItemList<Faculty>>('/faculties'),
  ]);

  return (
    <main className="w-full flex-1 px-4 py-6 sm:px-6 sm:py-10">
      <OnboardingWizard
        userId={me.id}
        profile={profile}
        faculties={faculties.items}
      />
    </main>
  );
}
