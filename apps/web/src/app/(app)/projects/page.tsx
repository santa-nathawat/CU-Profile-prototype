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
  Select,
} from '@/components/ui';
import { ProjectListCard } from '@/features/projects/ProjectListCard';
import { apiGet } from '@/lib/api';
import { plural } from '@/lib/format';
import { firstParam, listHref, parsePage } from '@/lib/nav';
import type {
  Category,
  ItemList,
  MyProject,
  Page,
  ProjectCard,
} from '@/lib/types';

export const metadata: Metadata = { title: 'Discover' };

const PAGE_SIZE = 12;
const PATH = '/projects';

export default async function DiscoverPage(props: PageProps<'/projects'>) {
  const params = await props.searchParams;
  const q = firstParam(params.q)?.trim() ?? '';
  const page = parsePage(params.page);

  const { items: categories } = await apiGet<ItemList<Category>>('/categories');
  const wanted = firstParam(params.category);
  const category = categories.some((item) => item.id === wanted) ? wanted : '';

  const [data, { items: myTeams }] = await Promise.all([
    apiGet<Page<ProjectCard>>(PATH, {
      q,
      category,
      page,
      pageSize: PAGE_SIZE,
    }),
    apiGet<ItemList<MyProject>>('/me/projects'),
  ]);
  const myTeam = (id: string) => myTeams.find((team) => team.id === id);

  const here = listHref(PATH, { q, category, page });
  const filtered = q !== '' || Boolean(category);

  return (
    <>
      <PageHeader
        title="Discover"
        description="Find a team that still needs someone like you."
        action={<ButtonLink href="/projects/new">Post a project</ButtonLink>}
      />

      <Form
        action={PATH}
        role="search"
        className="flex flex-col gap-2 sm:flex-row"
      >
        <label htmlFor="project-search" className="sr-only">
          Search projects
        </label>
        <Input
          key={`q-${q}`}
          id="project-search"
          name="q"
          type="search"
          defaultValue={q}
          placeholder="Search by name, description or role"
          className="flex-1"
        />
        <label htmlFor="project-category" className="sr-only">
          Category
        </label>
        <Select
          key={`category-${category}`}
          id="project-category"
          name="category"
          defaultValue={category}
          className="sm:max-w-64"
        >
          <option value="">All categories</option>
          {categories.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </Select>
        <Button type="submit" variant="secondary">
          Search
        </Button>
      </Form>

      {data.items.length > 0 ? (
        <>
          <p className="mt-6 text-sm text-ink-muted" aria-live="polite">
            {plural(data.total, 'project')} with open roles
            {filtered ? (
              <>
                {'. '}
                <Link href={PATH} className="link">
                  Clear search
                </Link>
              </>
            ) : null}
          </p>
          <ul className="mt-3 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {data.items.map((project) => (
              <li key={project.id}>
                <ProjectListCard
                  project={project}
                  from={here}
                  membership={
                    myTeam(project.id)
                      ? myTeam(project.id)?.isLead
                        ? 'lead'
                        : 'member'
                      : null
                  }
                />
              </li>
            ))}
          </ul>
          <Pagination
            page={data.page}
            pageSize={data.pageSize}
            total={data.total}
            hrefForPage={(next) => listHref(PATH, { q, category, page: next })}
          />
        </>
      ) : page > 1 ? (
        <EmptyState
          className="mt-6"
          title="No more projects on this page"
          action={
            <ButtonLink
              href={listHref(PATH, { q, category })}
              variant="secondary"
            >
              Back to the first page
            </ButtonLink>
          }
        />
      ) : filtered ? (
        <EmptyState
          className="mt-6"
          title="No projects match your search"
          description="Try other keywords or a different category. Only teams with open places show here."
          action={
            <ButtonLink href={PATH} variant="secondary">
              Clear search
            </ButtonLink>
          }
        />
      ) : (
        <EmptyState
          className="mt-6"
          title="No teams are recruiting right now"
          description="Be the first to post a project and find your teammates."
          action={<ButtonLink href="/projects/new">Post a project</ButtonLink>}
        />
      )}
    </>
  );
}
