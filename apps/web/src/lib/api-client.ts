import {
  toApiError,
  unreachableError,
  withQuery,
  type QueryValue,
} from './api-error';

async function send<T>(path: string, init: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api/v1${path}`, init);
  } catch {
    throw unreachableError();
  }
  if (!response.ok) throw await toApiError(response);
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

// The API rejects mutations without a JSON content type, even with no fields.
function json<T>(method: string, path: string, body?: unknown): Promise<T> {
  return send<T>(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body:
      method === 'DELETE'
        ? undefined
        : JSON.stringify(body === undefined ? {} : body),
  });
}

/** Browser calls, through the same-origin proxy. Throws ApiError on failure. */
export const clientApi = {
  get: <T>(path: string, query?: Record<string, QueryValue>) =>
    send<T>(withQuery(path, query), { method: 'GET' }),
  post: <T = void>(path: string, body?: unknown) => json<T>('POST', path, body),
  put: <T = void>(path: string, body?: unknown) => json<T>('PUT', path, body),
  patch: <T = void>(path: string, body?: unknown) =>
    json<T>('PATCH', path, body),
  delete: <T = void>(path: string) => json<T>('DELETE', path),
  /** Multipart. The browser sets the boundary, so no content type here. */
  upload: <T>(path: string, form: FormData) =>
    send<T>(path, { method: 'POST', body: form }),
};
