'use client';

import { useState } from 'react';
import { Alert, Badge, Button } from '@/components/ui';
import type { ProjectDetail } from '@/lib/types';
import { ApplyModal } from './ApplyModal';
import { placesLeft, projectEnded } from '../project-rules';

/** Why the whole team can't take an application right now, if it can't. */
function teamBlock(project: ProjectDetail): string | null {
  if (project.viewer.applyBlock) return project.viewer.applyBlock.message;
  if (projectEnded(project.status)) return 'This project has ended.';
  if (project.recruitmentStatus === 'closed') {
    return "The lead closed recruitment, so this team isn't taking applications right now.";
  }
  return null;
}

/** Roles with places left and Apply. A blocked action says why. */
export function ApplyRoles({ project }: { project: ProjectDetail }) {
  const [applyingFor, setApplyingFor] = useState<string | null>(null);
  const block = teamBlock(project);

  return (
    <>
      {block ? (
        <Alert tone="warning" className="mb-4">
          {block}
        </Alert>
      ) : null}
      <ul className="divide-y divide-line">
        {project.roles.map((role) => {
          const full = role.available === 0;
          return (
            <li
              key={role.id}
              className="flex flex-wrap items-center gap-3 py-3 first:pt-0 last:pb-0"
            >
              <div className="min-w-0 flex-1">
                <p className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">{role.name}</span>
                  <Badge tone={full ? 'neutral' : 'success'}>
                    {placesLeft(role)}
                  </Badge>
                </p>
                {full ? (
                  <p className="text-sm text-ink-muted">
                    Every place in this role is taken, so you can&apos;t apply
                    for it.
                  </p>
                ) : null}
              </div>
              <Button
                size="sm"
                disabled={full || block !== null}
                onClick={() => setApplyingFor(role.id)}
                aria-label={`Apply for ${role.name}`}
              >
                Apply
              </Button>
            </li>
          );
        })}
      </ul>
      <ApplyModal
        project={project}
        roleId={applyingFor}
        onClose={() => setApplyingFor(null)}
      />
    </>
  );
}
