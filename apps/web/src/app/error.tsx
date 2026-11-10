'use client';

import { ErrorPanel } from '@/components/ErrorPanel';

// Catches failures in the (app) layout too, such as the API being down.
export default function RootError({ retry }: { retry: () => void }) {
  return (
    <main className="mx-auto max-w-xl px-4 py-16 sm:px-6">
      <ErrorPanel retry={retry} />
    </main>
  );
}
