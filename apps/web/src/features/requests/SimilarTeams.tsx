'use client';

import Link from 'next/link';
import { useState } from 'react';
import { Badge, Button } from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import { projectHref } from '@/lib/nav';
import type { ItemList, MyProject, ProjectCard } from '@/lib/types';

/** Loads on request, so a long request list doesn't make a call for every row. */
export function SimilarTeams({
  projectId,
  from,
}: {
  projectId: string;
  from: string;
}) {
  const [teams, setTeams] = useState<ProjectCard[] | null>(null);
  const [mine, setMine] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);

  async function load() {
    setBusy(true);
    try {
      const [similar, myTeams] = await Promise.all([
        clientApi.get<ItemList<ProjectCard>>(`/projects/${projectId}/similar`),
        clientApi.get<ItemList<MyProject>>('/me/projects'),
      ]);
      setMine(new Set(myTeams.items.map((team) => team.id)));
      setTeams(similar.items);
    } catch {
      // A deleted team has no similar list. Search is still a useful next step.
      setTeams([]);
    } finally {
      setBusy(false);
    }
  }

  if (!teams) {
    return (
      <Button variant="secondary" size="sm" loading={busy} onClick={load}>
        See similar teams
      </Button>
    );
  }

  if (teams.length === 0) {
    return (
      <p className="basis-full text-sm text-ink-muted">
        No similar teams are recruiting right now.{' '}
        <Link href="/projects" className="link">
          Search all projects
        </Link>
      </p>
    );
  }

  return (
    <div className="basis-full">
      <p className="text-sm font-medium">Similar teams recruiting now</p>
      <ul className="mt-1 space-y-1 text-sm">
        {teams.map((team) => (
          <li key={team.id}>
            <Link href={projectHref(team.id, from)} className="link">
              {team.name}
            </Link>{' '}
            <span className="text-ink-subtle">{team.category.name}</span>
            {mine.has(team.id) ? (
              <Badge className="ml-1.5">Your team</Badge>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}
