'use client';

import Link from 'next/link';
import { useEffect, useId, useRef, useState } from 'react';
import { Avatar } from '@/components/ui';
import { useSignOut } from './use-sign-out';

const itemClass =
  'block w-full rounded-md px-3 py-2 text-left text-sm hover:bg-sunken';

export function AccountMenu({
  userId,
  name,
  email,
  profileComplete,
}: {
  userId: string;
  name: string;
  email: string;
  profileComplete: boolean;
}) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const menuId = useId();
  const { busy, signOut } = useSignOut();

  useEffect(() => {
    if (!open) return;
    function onPointerDown(event: PointerEvent) {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setOpen(false);
        button.current?.focus();
      }
    }
    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('pointerdown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  return (
    <div ref={root} className="relative">
      <button
        ref={button}
        type="button"
        aria-expanded={open}
        aria-controls={menuId}
        aria-label="Account menu"
        onClick={() => setOpen(!open)}
        className="flex size-11 items-center justify-center rounded-lg hover:bg-sunken"
      >
        <Avatar name={name} size="sm" />
      </button>
      <div
        id={menuId}
        hidden={!open}
        className="absolute right-0 z-30 mt-1 w-64 max-w-[calc(100vw-2rem)] rounded-lg border border-line-strong bg-surface p-1.5"
      >
        <div className="border-b border-line px-3 py-2">
          <p className="text-sm font-medium">{name}</p>
          <p className="text-xs text-ink-subtle">{email}</p>
        </div>
        <div className="pt-1">
          {profileComplete ? (
            <>
              <Link
                href={`/people/${userId}`}
                onClick={() => setOpen(false)}
                className={itemClass}
              >
                View my profile
              </Link>
              <Link
                href="/settings"
                onClick={() => setOpen(false)}
                className={itemClass}
              >
                Settings
              </Link>
            </>
          ) : (
            <Link
              href="/onboarding"
              onClick={() => setOpen(false)}
              className={itemClass}
            >
              Complete your profile
            </Link>
          )}
          <button
            type="button"
            disabled={busy}
            onClick={signOut}
            className={itemClass}
          >
            {busy ? 'Signing out' : 'Sign out'}
          </button>
        </div>
      </div>
    </div>
  );
}
