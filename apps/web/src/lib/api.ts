import 'server-only';
import { cookies } from 'next/headers';
import { notFound, redirect } from 'next/navigation';
import {
  ApiError,
  toApiError,
  unreachableError,
  withQuery,
  type QueryValue,
} from './api-error';

// Read at request time so one image can run against any API.
function apiBase(): string {
  return process.env.API_URL ?? 'http://localhost:3001';
}

type ServerRequest = {
  method?: string;
  query?: Record<string, QueryValue>;
  body?: unknown;
  timeoutMs?: number;
};

/** Calls the API with the visitor's cookie. Throws ApiError on any failure. */
export async function apiRequest<T>(
  path: string,
  { method = 'GET', query, body, timeoutMs }: ServerRequest = {},
): Promise<T> {
  const headers: Record<string, string> = {
    cookie: (await cookies()).toString(),
  };
  if (body !== undefined) headers['content-type'] = 'application/json';

  let response: Response;
  try {
    response = await fetch(`${apiBase()}/api/v1${withQuery(path, query)}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      cache: 'no-store',
      signal: timeoutMs ? AbortSignal.timeout(timeoutMs) : undefined,
    });
  } catch {
    throw unreachableError();
  }
  if (!response.ok) throw await toApiError(response);
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

/** For pages behind the guard: an expired session goes back to sign-in. */
export async function apiGet<T>(
  path: string,
  query?: Record<string, QueryValue>,
): Promise<T> {
  try {
    return await apiRequest<T>(path, { query });
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) redirect('/sign-in');
    throw error;
  }
}

/** Like apiGet, but a 404 renders the nearest not-found page. */
export async function apiGetOrNotFound<T>(
  path: string,
  query?: Record<string, QueryValue>,
): Promise<T> {
  try {
    return await apiGet<T>(path, query);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }
}
