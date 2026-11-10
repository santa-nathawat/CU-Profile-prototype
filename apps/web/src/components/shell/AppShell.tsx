import { Suspense } from 'react';
import { NavTracker } from './NavTracker';
import { NoticeToast } from './NoticeToast';

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-1 flex-col">
      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 sm:px-6 sm:py-8">
        {children}
      </main>
      <Suspense>
        <NavTracker />
      </Suspense>
      <NoticeToast />
    </div>
  );
}
