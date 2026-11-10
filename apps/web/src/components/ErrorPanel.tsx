'use client';

import { Button, EmptyState } from './ui';

export function ErrorPanel({ retry }: { retry: () => void }) {
  return (
    <EmptyState
      title="This page didn't load"
      description="The server may be waking up or having a problem. Please try again in a moment."
      action={<Button onClick={() => retry()}>Try again</Button>}
    />
  );
}
