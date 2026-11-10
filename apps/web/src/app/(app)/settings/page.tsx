import type { Metadata } from 'next';
import { PageHeader } from '@/components/PageHeader';
import { Section } from '@/components/ui';
import { CvManager } from '@/features/profiles/CvManager';
import { AvailabilityForm } from '@/features/settings/AvailabilityForm';
import { CloseAccount } from '@/features/settings/CloseAccount';
import { ProfileForm } from '@/features/settings/ProfileForm';
import { apiGet } from '@/lib/api';
import type { Faculty, ItemList, MyProfile } from '@/lib/types';

export const metadata: Metadata = { title: 'Settings' };

export default async function SettingsPage() {
  const [profile, faculties] = await Promise.all([
    apiGet<MyProfile>('/me/profile'),
    apiGet<ItemList<Faculty>>('/faculties'),
  ]);

  return (
    <div className="mx-auto max-w-2xl">
      <PageHeader title="Settings" />
      <div className="space-y-4">
        <Section
          title="Profile"
          description="Other signed-in students see this on your profile."
        >
          <ProfileForm profile={profile} faculties={faculties.items} />
        </Section>
        <Section title="CV" description="PDF only, up to 5 MB.">
          <CvManager
            userId={profile.userId}
            fileName={profile.cv.fileName}
            shared={profile.cvShared}
          />
        </Section>
        <Section title="Weekly availability">
          <AvailabilityForm initial={profile.availability} />
        </Section>
        <Section title="Account">
          <p className="text-sm font-medium">Sign-in email</p>
          <p>{profile.email}</p>
          <p className="mt-1 text-sm text-ink-subtle">
            Only you can see this. It never shows on your profile.
          </p>
          <div className="mt-6 border-t border-line pt-4">
            <p className="text-sm font-medium">Close account</p>
            <p className="mt-1 mb-3 text-sm text-ink-muted">
              Hides your profile and ratings and ends your place on teams.
            </p>
            <CloseAccount />
          </div>
        </Section>
      </div>
    </div>
  );
}
