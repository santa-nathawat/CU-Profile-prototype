import { withQuery, type QueryValue } from './api-error';

type Param = string | string[] | undefined;

export function firstParam(value: Param): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

export function parsePage(value: Param): number {
  const page = Number(firstParam(value));
  return Number.isInteger(page) && page > 0 ? page : 1;
}

/** Builds an in-app URL, dropping empty values and page 1. */
export function listHref(
  path: string,
  query: Record<string, QueryValue>,
): string {
  const { page, ...rest } = query;
  return withQuery(path, { ...rest, page: page === 1 ? undefined : page });
}

/** Only same-site paths may be used as a return target. */
export function safeFrom(value: Param): string | null {
  const from = firstParam(value);
  if (!from || !from.startsWith('/') || from.startsWith('//')) return null;
  if (from.includes('\\')) return null;
  return from;
}

/**
 * Link to a profile from any list. `from` is the list's current URL including
 * its search, so the profile's Back control can return to the same results.
 */
export function profileHref(userId: string, from?: string): string {
  return withQuery(`/people/${userId}`, { from });
}

export function backLabel(from: string | null): string {
  const path = from?.split('?')[0] ?? '';
  if (path === '/people') return 'Back to People';
  if (path === '/projects') return 'Back to Discover';
  if (path.startsWith('/projects/')) return 'Back to project';
  if (path === '/requests') return 'Back to Requests';
  if (path.startsWith('/requests/')) return 'Back to request';
  if (path === '/my-teams') return 'Back to My teams';
  if (path === '/notifications') return 'Back to Inbox';
  return 'Back';
}

export function projectHref(projectId: string, from?: string): string {
  return withQuery(`/projects/${projectId}`, { from });
}

export function requestHref(requestId: string, from?: string): string {
  return withQuery(`/requests/${requestId}`, { from });
}

/** Where a student lands after signing in. */
export function homeFor(me: { profileComplete: boolean }): string {
  return me.profileComplete ? '/projects' : '/onboarding';
}

/**
 * A full page load for when the signed-in person changes, so nothing from the
 * previous session stays in the client router cache.
 */
export function reloadTo(path: string): void {
  window.location.assign(path);
}
