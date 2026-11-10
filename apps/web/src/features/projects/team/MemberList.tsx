import Link from 'next/link';
import { Avatar, Badge } from '@/components/ui';
import { formatDate } from '@/lib/format';
import { profileHref } from '@/lib/nav';
import type { Member } from '@/lib/types';

export function MemberList({
  members,
  from,
  actions,
}: {
  members: Member[];
  from: string;
  /** Extra controls on a member's row, such as the lead's team actions. */
  actions?: (member: Member) => React.ReactNode;
}) {
  return (
    <ul className="divide-y divide-line">
      {members.map((member) => (
        <li
          key={member.userId}
          className="flex flex-wrap items-center gap-3 py-3 first:pt-0 last:pb-0"
        >
          <Avatar name={member.displayName} size="sm" />
          <div className="min-w-0 flex-1">
            <p className="flex flex-wrap items-center gap-1.5">
              <Link
                href={profileHref(member.userId, from)}
                className="font-medium underline-offset-2 hover:underline"
              >
                {member.displayName}
              </Link>
              {member.isLead ? <Badge tone="accent">Lead</Badge> : null}
            </p>
            <p className="text-sm text-ink-muted">
              {member.roleName}
              {member.faculty ? `, ${member.faculty}` : ''}. Joined{' '}
              {formatDate(member.joinedAt)}
            </p>
          </div>
          {actions?.(member)}
        </li>
      ))}
    </ul>
  );
}
