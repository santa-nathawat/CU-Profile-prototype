import { Badge } from '@/components/ui';
import type { Role } from '@/lib/types';
import { placesLeft } from './project-rules';

/** Read-only roles, for people who can't apply or change them. */
export function RoleSummary({ roles }: { roles: Role[] }) {
  return (
    <ul className="divide-y divide-line">
      {roles.map((role) => (
        <li
          key={role.id}
          className="flex items-center justify-between gap-3 py-3 first:pt-0 last:pb-0"
        >
          <span className="min-w-0 font-medium">{role.name}</span>
          <Badge tone={role.available > 0 ? 'success' : 'neutral'}>
            {placesLeft(role)}
          </Badge>
        </li>
      ))}
    </ul>
  );
}
