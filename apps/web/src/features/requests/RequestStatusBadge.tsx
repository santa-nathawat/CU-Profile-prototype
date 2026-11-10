'use client';

import { Badge } from '@/components/ui';
import type { RecruitmentRequest } from '@/lib/types';
import { useNow } from '@/lib/use-now';
import { STATUS } from './request-status';

/** The API expires requests when they're read, so an open page catches up here. */
export function RequestStatusBadge({
  request,
}: {
  request: Pick<RecruitmentRequest, 'status' | 'expiresAt'>;
}) {
  const now = useNow();
  const pastDue =
    request.status === 'pending' &&
    now !== null &&
    now >= new Date(request.expiresAt).getTime();
  const { label, tone } = STATUS[pastDue ? 'expired' : request.status];
  return <Badge tone={tone}>{label}</Badge>;
}
