'use client';

import { ErrorPanel } from '@/components/ErrorPanel';

export default function AppError({ retry }: { retry: () => void }) {
  return <ErrorPanel retry={retry} />;
}
