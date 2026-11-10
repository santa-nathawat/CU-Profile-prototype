import { ButtonLink } from '@/components/ui';
import type { Me } from '@/lib/types';
import { AccountMenu } from './AccountMenu';
import { InboxBell } from './InboxBell';
import { Logo } from './Logo';
import { TopNav } from './TopNav';

export function SiteHeader({ me }: { me: Me | null }) {
  return (
    <header className="z-20 shrink-0 border-b border-line bg-surface md:sticky md:top-0">
      <div className="mx-auto grid max-w-6xl grid-cols-[1fr_auto] items-center px-4 sm:px-6 md:grid-cols-[auto_1fr_auto] md:gap-x-8">
        <div className="flex min-h-14 items-center">
          <Logo href="/" />
        </div>
        <TopNav className="col-span-2 row-start-2 -mx-2 flex border-t border-line md:col-span-1 md:col-start-2 md:row-start-1 md:mx-0 md:self-stretch md:border-0" />
        <div className="col-start-2 row-start-1 flex items-center justify-end gap-1 md:col-start-3">
          {me ? (
            <>
              {me.profileComplete ? (
                <InboxBell unread={me.unreadNotifications} />
              ) : null}
              <AccountMenu
                userId={me.id}
                name={me.displayName ?? me.email}
                email={me.email}
                profileComplete={me.profileComplete}
              />
            </>
          ) : (
            <ButtonLink href="/sign-in" size="sm" variant="secondary">
              Sign in
            </ButtonLink>
          )}
        </div>
      </div>
    </header>
  );
}
