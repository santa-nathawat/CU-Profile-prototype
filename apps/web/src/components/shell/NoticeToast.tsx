'use client';

import { useEffect } from 'react';
import { usePathname } from 'next/navigation';
import { Alert } from '@/components/ui';
import { CloseIcon } from '@/components/ui/icons';
import { dismissNotice, useNotice } from '@/lib/notice';

export function NoticeToast() {
  const message = useNotice();
  const pathname = usePathname();

  useEffect(() => dismissNotice, [pathname]);

  if (!message) return null;
  return (
    <div className="fixed inset-x-4 bottom-4 z-30 mx-auto max-w-lg">
      <Alert tone="error" className="flex items-start justify-between gap-3">
        <span className="min-w-0">{message}</span>
        <button
          type="button"
          onClick={dismissNotice}
          aria-label="Dismiss"
          className="-mt-0.5 -mr-1 rounded p-0.5 hover:bg-danger/10"
        >
          <CloseIcon className="size-4" />
        </button>
      </Alert>
    </div>
  );
}
