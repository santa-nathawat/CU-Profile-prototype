'use client';

import { useState } from 'react';
import { Badge, Button, Card, Field, FormAlert, Input } from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import { homeFor, reloadTo } from '@/lib/nav';
import type { DemoAccount, Me } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';

const CHULA_DOMAIN = '@student.chula.ac.th';
const ONLY_CHULA = `Only Chula student emails are accepted. Use an address ending in ${CHULA_DOMAIN}.`;

function isChulaEmail(email: string): boolean {
  const lower = email.trim().toLowerCase();
  return lower.endsWith(CHULA_DOMAIN) && lower.length > CHULA_DOMAIN.length;
}

export function DemoSignIn({ accounts }: { accounts: DemoAccount[] }) {
  const form = useApiForm();
  const [email, setEmail] = useState('');
  const [pending, setPending] = useState<string | null>(null);

  async function signIn(address: string) {
    setPending(address);
    const result = await form.run(() =>
      clientApi.post<Me>('/auth/demo-sign-in', { email: address.trim() }),
    );
    if (result.ok) {
      // Full load, so nothing from a previous person stays in the client cache.
      reloadTo(homeFor(result.data));
    } else {
      setPending(null);
    }
  }

  function submit(event: React.FormEvent) {
    event.preventDefault();
    if (!isChulaEmail(email)) {
      form.fail({ email: ONLY_CHULA });
      return;
    }
    void signIn(email);
  }

  return (
    <Card>
      <div className="flex items-center gap-2">
        <h2 className="text-lg font-semibold">Demo sign-in</h2>
        <Badge tone="warning">Demo only</Badge>
      </div>
      <p className="mt-1 text-sm text-ink-muted">
        For trying out the app. There is no verification here, so anyone can use
        any address.
      </p>

      <form onSubmit={submit} noValidate className="mt-4 space-y-3">
        <Field label="Chula email" error={form.fields.email}>
          <Input
            type="email"
            inputMode="email"
            autoComplete="email"
            placeholder={`name${CHULA_DOMAIN}`}
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </Field>
        <Button
          type="submit"
          variant="secondary"
          className="w-full"
          loading={pending === email && form.busy}
          disabled={form.busy}
        >
          Continue with this email
        </Button>
      </form>

      <FormAlert form={form} className="mt-3" />

      {accounts.length > 0 ? (
        <div className="mt-6">
          <h3 className="text-sm font-semibold text-ink-muted">
            Or pick a demo account
          </h3>
          <ul className="mt-2 divide-y divide-line">
            {accounts.map((account) => (
              <li key={account.email}>
                <button
                  type="button"
                  disabled={form.busy}
                  onClick={() => void signIn(account.email)}
                  className="flex w-full items-center justify-between gap-3 rounded-md px-2 py-2.5 text-left hover:bg-sunken disabled:opacity-60"
                >
                  <span className="min-w-0">
                    <span className="block font-medium">
                      {account.displayName}
                    </span>
                    <span className="block text-sm text-ink-subtle">
                      {account.note}
                    </span>
                  </span>
                  <span className="shrink-0 text-sm font-medium text-accent-ink">
                    {pending === account.email ? 'Signing in' : 'Sign in'}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </Card>
  );
}
