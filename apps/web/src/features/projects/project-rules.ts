import type { ProjectStatus, Role } from '@/lib/types';

export function projectEnded(status: ProjectStatus): boolean {
  return status === 'completed' || status === 'cancelled';
}

/** "1 of 2 left", or "Full". Capacity already counts the lead's own place. */
export function placesLeft(role: Role): string {
  return role.available > 0
    ? `${role.available} of ${role.capacity} left`
    : 'Full';
}
