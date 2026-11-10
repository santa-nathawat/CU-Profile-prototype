import type { Metadata } from 'next';
import { BackLink } from '@/components/BackLink';
import {
  Avatar,
  Badge,
  ButtonLink,
  RatingSummary,
  Section,
} from '@/components/ui';
import {
  AvailabilityView,
  NOT_SPECIFIED,
} from '@/features/profiles/AvailabilityView';
import { InviteAction } from '@/features/profiles/InviteAction';
import {
  HistorySection,
  RatingsSection,
  TeammatesSection,
} from '@/features/profiles/ProfileSections';
import { apiGetOrNotFound } from '@/lib/api';
import { facultyLine } from '@/lib/format';
import { profileHref, safeFrom } from '@/lib/nav';
import type {
  ItemList,
  Profile,
  ProfileHistory,
  ProfileRatings,
  TeammateRating,
} from '@/lib/types';

export const metadata: Metadata = { title: 'Student profile' };

export default async function ProfilePage(props: PageProps<'/people/[id]'>) {
  const { id } = await props.params;
  const from = safeFrom((await props.searchParams).from);

  const [profile, history, ratings, teammates] = await Promise.all([
    apiGetOrNotFound<Profile>(`/profiles/${id}`),
    apiGetOrNotFound<ProfileHistory>(`/profiles/${id}/history`),
    apiGetOrNotFound<ProfileRatings>(`/profiles/${id}/ratings`),
    apiGetOrNotFound<ItemList<TeammateRating>>(`/profiles/${id}/teammates`),
  ]);

  // Links out of this profile come back here, with the original list kept.
  const here = profileHref(id, from ?? undefined);
  const invitable = profile.viewerActions.invitableProjects;

  return (
    <div className="mx-auto max-w-4xl">
      <BackLink from={from} fallback="/people" />

      <header className="mt-3 flex flex-col gap-4 sm:flex-row sm:items-start">
        <Avatar name={profile.displayName} size="lg" />
        <div className="min-w-0 flex-1">
          <h1 className="text-2xl font-semibold">{profile.displayName}</h1>
          <p className="text-ink-muted">
            {facultyLine(profile.faculty?.name, profile.year) || NOT_SPECIFIED}
          </p>
          {profile.headline ? <p className="mt-2">{profile.headline}</p> : null}
          <div className="mt-2">
            <RatingSummary rating={profile.rating} />
          </div>
          <p className="mt-2 text-sm text-ink-subtle">
            {profile.isSelf
              ? 'Your email stays private. Nobody else can see it.'
              : "This student's email stays private."}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {profile.isSelf ? (
            <ButtonLink href="/settings" variant="secondary">
              Edit profile
            </ButtonLink>
          ) : null}
          {invitable.length > 0 ? (
            <InviteAction
              studentId={profile.userId}
              studentName={profile.displayName}
              projects={invitable}
            />
          ) : null}
        </div>
      </header>

      <div className="mt-6 grid gap-4 md:grid-cols-2">
        <Section title="Skills">
          <ul className="flex flex-wrap gap-1.5">
            {profile.skills.map((skill) => (
              <li key={skill}>
                <Badge>{skill}</Badge>
              </li>
            ))}
          </ul>
        </Section>
        <Section title="Working style">
          {profile.workingStyle ? (
            <p className="whitespace-pre-line">{profile.workingStyle}</p>
          ) : (
            <p className="text-ink-subtle">{NOT_SPECIFIED}</p>
          )}
        </Section>
        <Section title="Weekly availability">
          <AvailabilityView slots={profile.availability} />
        </Section>
        <Section title="CV">
          {profile.cv.available ? (
            <a
              href={`/api/v1/users/${profile.userId}/cv`}
              target="_blank"
              rel="noopener"
              className="link"
            >
              {profile.cv.fileName ?? 'Open CV'}
            </a>
          ) : (
            <p className="text-ink-subtle">{NOT_SPECIFIED}</p>
          )}
        </Section>
      </div>

      <div className="mt-4 space-y-4">
        <HistorySection history={history} from={here} />
        <TeammatesSection teammates={teammates.items} from={here} />
        <RatingsSection ratings={ratings} from={here} />
      </div>
    </div>
  );
}
