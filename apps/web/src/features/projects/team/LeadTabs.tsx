import { ButtonLink, Card, EmptyState, Tabs } from '@/components/ui';
import { listHref } from '@/lib/nav';
import type { ProjectDetail, RecruitmentRequest } from '@/lib/types';
import { ApplicantCard } from '../recruiting/ApplicantCard';
import { InviteRow } from '../recruiting/InviteRow';
import { MemberList } from './MemberList';
import { MemberActions } from './TeamActions';

export type LeadTab = 'applicants' | 'invites' | 'members';

export function parseLeadTab(value: string | undefined): LeadTab {
  return value === 'invites' || value === 'members' ? value : 'applicants';
}

export function LeadTabs({
  project,
  requests,
  tab,
  backTo,
  here,
}: {
  project: ProjectDetail;
  requests: RecruitmentRequest[];
  tab: LeadTab;
  /** The team page's own `from`, kept across tab changes. */
  backTo: string | undefined;
  here: string;
}) {
  const path = `/projects/${project.id}`;
  const applicants = requests.filter(
    (request) => request.type === 'application' && request.status === 'pending',
  );
  const invitations = requests.filter(
    (request) => request.type === 'invitation',
  );
  const pendingInvites = invitations.filter(
    (request) => request.status === 'pending',
  );
  const others = invitations.filter((request) => request.status !== 'pending');

  const tabHref = (next: LeadTab) =>
    listHref(path, {
      from: backTo,
      tab: next === 'applicants' ? undefined : next,
    });

  return (
    <section aria-label="Team requests and members">
      <Tabs
        label="Team requests and members"
        items={[
          {
            label: 'Applicants',
            count: applicants.length,
            active: tab === 'applicants',
            href: tabHref('applicants'),
            scroll: false,
          },
          {
            label: 'Invites sent',
            count: pendingInvites.length,
            active: tab === 'invites',
            href: tabHref('invites'),
            scroll: false,
          },
          {
            label: 'Members',
            count: project.members.length,
            active: tab === 'members',
            href: tabHref('members'),
            scroll: false,
          },
        ]}
      />
      <div className="mt-4">
        {tab === 'applicants' ? (
          applicants.length > 0 ? (
            <>
              <p className="mb-3 text-sm text-ink-muted">
                Approving the last place in a role closes the other pending
                requests for that role.
              </p>
              <ul className="grid gap-4 md:grid-cols-2">
                {applicants.map((request) => (
                  <li key={request.id}>
                    <ApplicantCard request={request} from={here} />
                  </li>
                ))}
              </ul>
            </>
          ) : (
            <EmptyState
              title="No applicants waiting"
              description="New applications show here. You have 48 hours to answer each one."
            />
          )
        ) : null}
        {tab === 'invites' ? (
          invitations.length > 0 ? (
            <Card>
              <ul className="divide-y divide-line">
                {[...pendingInvites, ...others].map((request) => (
                  <InviteRow key={request.id} request={request} from={here} />
                ))}
              </ul>
            </Card>
          ) : (
            <EmptyState
              title="No invites sent yet"
              description="Find someone with the right skills and invite them to a role."
              action={
                <ButtonLink href="/people" variant="secondary">
                  Find people
                </ButtonLink>
              }
            />
          )
        ) : null}
        {tab === 'members' ? (
          <Card>
            <MemberList
              members={project.members}
              from={here}
              actions={(member) => (
                <MemberActions project={project} member={member} />
              )}
            />
          </Card>
        ) : null}
      </div>
    </section>
  );
}
