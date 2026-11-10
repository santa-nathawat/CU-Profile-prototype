import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { Alert, Button, Card } from '@/components/ui';
import { DemoSignIn } from '@/features/auth/DemoSignIn';
import { apiRequest } from '@/lib/api';
import { ApiError } from '@/lib/api-error';
import { getMeIfReachable } from '@/lib/auth';
import { homeFor } from '@/lib/nav';
import type { DemoAccount, ItemList } from '@/lib/types';

export const metadata: Metadata = { title: 'Sign in' };

type SignInState =
  | { kind: 'demo'; accounts: DemoAccount[] }
  | { kind: 'not_available' }
  | { kind: 'unreachable' };

async function loadSignIn(): Promise<SignInState> {
  try {
    const { items } = await apiRequest<ItemList<DemoAccount>>(
      '/auth/demo-accounts',
      { timeoutMs: 5000 },
    );
    return { kind: 'demo', accounts: items };
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      return { kind: 'not_available' };
    }
    return { kind: 'unreachable' };
  }
}

export default async function SignInPage() {
  const me = await getMeIfReachable();
  if (me) redirect(homeFor(me));

  const state = await loadSignIn();

  return (
    <div className="mx-auto flex w-full flex-1 max-w-md flex-col px-4 py-6 sm:px-6">
      <main className="space-y-4 sm:mt-8">
        <Card>
          <h1 className="text-xl font-semibold">Sign in to CU-Profile</h1>
          <p className="mt-1 text-sm text-ink-muted">
            For Chulalongkorn University students. Your email stays private.
          </p>
          <Button className="mt-5 w-full" disabled>
            Sign in with Chula
          </Button>
          {state.kind === 'demo' ? (
            <p className="mt-2 text-center text-sm text-ink-subtle">
              Chula sign-in is coming soon. Use the demo sign-in below for now.
            </p>
          ) : null}
          {state.kind === 'not_available' ? (
            <Alert className="mt-4">
              Sign-in isn&apos;t available yet. Please check back soon.
            </Alert>
          ) : null}
          {state.kind === 'unreachable' ? (
            <Alert tone="error" className="mt-4">
              We can&apos;t reach the server right now, so sign-in is
              unavailable. Please try again in a moment.
            </Alert>
          ) : null}
        </Card>
        {state.kind === 'demo' ? (
          <DemoSignIn accounts={state.accounts} />
        ) : null}
      </main>
    </div>
  );
}
