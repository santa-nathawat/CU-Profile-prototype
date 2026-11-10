import type { Metadata } from 'next';
import Form from 'next/form';
import Link from 'next/link';
import { PageHeader } from '@/components/PageHeader';
import {
  Button,
  ButtonLink,
  EmptyState,
  Input,
  Pagination,
} from '@/components/ui';
import { PersonCard } from '@/features/profiles/PersonCard';
import { apiGet } from '@/lib/api';
import { ApiError } from '@/lib/api-error';
import { getMe } from '@/lib/auth';
import { plural } from '@/lib/format';
import { firstParam, listHref, parsePage } from '@/lib/nav';
import type {
  InvitableProject,
  ItemList,
  MyProject,
  Page,
  ProfileCard,
} from '@/lib/types';

export const metadata: Metadata = { title: 'People' };

const PAGE_SIZE = 12;
const PATH = '/people';

/**
 * One call instead of each profile's viewerActions. Invite on a card is an extra,
 * so the list still works if this fails.
 */
async function loadInvitableProjects(): Promise<InvitableProject[]> {
  try {
    const { items } = await apiGet<ItemList<MyProject>>('/me/projects');
    return items
      .filter((team) => team.isLead && team.recruitmentStatus === 'open')
      .map(({ id, name, roles }) => ({ id, name, roles }));
  } catch (error) {
    if (error instanceof ApiError) return [];
    throw error;
  }
}

export default async function PeoplePage(props: PageProps<'/people'>) {
  const params = await props.searchParams;
  const q =
    firstParam(params.q)?.trim() || firstParam(params.skill)?.trim() || '';
  const page = parsePage(params.page);

  const [data, invitableProjects, me] = await Promise.all([
    apiGet<Page<ProfileCard>>('/profiles', {
      q,
      page,
      pageSize: PAGE_SIZE,
    }),
    loadInvitableProjects(),
    getMe(),
  ]);

  const here = listHref(PATH, { q, page });

  return (
    <>
      <PageHeader
        title="People"
        description="Find students by name or skill."
      />

      <Form
        action={PATH}
        role="search"
        className="flex flex-col gap-2 sm:max-w-xl sm:flex-row"
      >
        <label htmlFor="people-search" className="sr-only">
          Search people
        </label>
        <Input
          key={q}
          id="people-search"
          name="q"
          type="search"
          defaultValue={q}
          placeholder="Search by name or skill"
          className="flex-1"
        />
        <Button type="submit" variant="secondary">
          Search
        </Button>
      </Form>

      {data.items.length > 0 ? (
        <>
          <p className="mt-6 text-sm text-ink-muted" aria-live="polite">
            {plural(data.total, 'person', 'people')}
            {q ? ` matching "${q}"` : ''}
            {q ? (
              <>
                {'. '}
                <Link href={PATH} className="link">
                  Clear search
                </Link>
              </>
            ) : null}
          </p>
          <ul className="mt-3 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {data.items.map((person) => (
              <li key={person.userId}>
                <PersonCard
                  person={person}
                  from={here}
                  highlightSkill={q}
                  invitableProjects={
                    person.userId === me?.id ? [] : invitableProjects
                  }
                />
              </li>
            ))}
          </ul>
          <Pagination
            page={data.page}
            pageSize={data.pageSize}
            total={data.total}
            hrefForPage={(next) => listHref(PATH, { q, page: next })}
          />
        </>
      ) : q ? (
        <EmptyState
          className="mt-6"
          title={`No one matches "${q}"`}
          description="Try another name or skill, or post a project so people can find you."
          action={
            <div className="flex flex-wrap justify-center gap-2">
              <ButtonLink href={PATH} variant="secondary">
                Clear search
              </ButtonLink>
              <ButtonLink href="/projects/new">Post a project</ButtonLink>
            </div>
          }
        />
      ) : page > 1 ? (
        <EmptyState
          className="mt-6"
          title="No more people on this page"
          action={
            <ButtonLink href={PATH} variant="secondary">
              Back to the first page
            </ButtonLink>
          }
        />
      ) : (
        <EmptyState
          className="mt-6"
          title="No student profiles yet"
          description="Profiles appear here once students add a name, a faculty or school and a skill."
        />
      )}
    </>
  );
}
