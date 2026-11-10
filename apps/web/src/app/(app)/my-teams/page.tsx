import type { Metadata } from 'next';
import { PageHeader } from '@/components/PageHeader';
import { ButtonLink, EmptyState } from '@/components/ui';
import { MyTeamCard } from '@/features/projects/MyTeamCard';
import { projectEnded } from '@/features/projects/project-rules';
import { apiGet } from '@/lib/api';
import type { ItemList, MyProject } from '@/lib/types';

export const metadata: Metadata = { title: 'My teams' };

const PATH = '/my-teams';

function TeamGroup({ title, teams }: { title: string; teams: MyProject[] }) {
  if (teams.length === 0) return null;
  return (
    <section className="mt-6 first:mt-0">
      <h2 className="text-lg font-semibold">{title}</h2>
      <ul className="mt-3 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {teams.map((team) => (
          <li key={team.id}>
            <MyTeamCard project={team} from={PATH} />
          </li>
        ))}
      </ul>
    </section>
  );
}

export default async function MyTeamsPage() {
  const { items } = await apiGet<ItemList<MyProject>>('/me/projects');
  const active = items.filter((team) => !projectEnded(team.status));

  return (
    <>
      <PageHeader
        title="My teams"
        description="Teams you lead or belong to."
        action={<ButtonLink href="/projects/new">Post a project</ButtonLink>}
      />
      {items.length > 0 ? (
        <>
          <TeamGroup
            title="Leading"
            teams={active.filter((team) => team.isLead)}
          />
          <TeamGroup
            title="Member"
            teams={active.filter((team) => !team.isLead)}
          />
          <TeamGroup
            title="Ended"
            teams={items.filter((team) => projectEnded(team.status))}
          />
        </>
      ) : (
        <EmptyState
          title="You're not on a team yet"
          description="Find a team that needs someone like you, or post your own project."
          action={
            <div className="flex flex-wrap justify-center gap-2">
              <ButtonLink href="/projects" variant="secondary">
                Discover projects
              </ButtonLink>
              <ButtonLink href="/projects/new">Post a project</ButtonLink>
            </div>
          }
        />
      )}
    </>
  );
}
