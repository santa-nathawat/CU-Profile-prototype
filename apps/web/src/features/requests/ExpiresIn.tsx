'use client';

import { expiresInLabel, formatDateTime } from '@/lib/format';
import { useNow } from '@/lib/use-now';

/** A live countdown. `withDate` adds the deadline itself in Bangkok time. */
export function ExpiresIn({
  expiresAt,
  withDate = false,
  className = '',
}: {
  expiresAt: string;
  withDate?: boolean;
  className?: string;
}) {
  const now = useNow();
  const deadline = formatDateTime(expiresAt);

  if (now === null) {
    return <span className={className}>Expires {deadline}</span>;
  }
  const label = expiresInLabel(expiresAt, now);
  if (!label) return <span className={className}>Expired {deadline}</span>;
  return (
    <span className={className} title={deadline}>
      {label[0].toUpperCase() + label.slice(1)}
      {withDate ? ` (${deadline})` : ''}
    </span>
  );
}
