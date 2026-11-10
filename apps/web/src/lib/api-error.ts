export type FieldErrors = Record<string, string>;

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly fields: FieldErrors = {},
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export const UNREACHABLE_MESSAGE =
  "We can't reach the server right now. Please try again in a moment.";

export function unreachableError(): ApiError {
  return new ApiError(503, 'api_unreachable', UNREACHABLE_MESSAGE);
}

export async function toApiError(response: Response): Promise<ApiError> {
  try {
    const { error } = await response.json();
    if (error?.code && error?.message) {
      return new ApiError(
        response.status,
        error.code,
        error.message,
        error.fields ?? {},
      );
    }
  } catch {
    // Not JSON: a gateway or a crash. Fall through to the generic message.
  }
  return new ApiError(
    response.status,
    'unexpected_response',
    response.status === 413
      ? 'That file is too large.'
      : 'Something went wrong. Please try again.',
  );
}

export type QueryValue = string | number | boolean | null | undefined;

export function withQuery(
  path: string,
  query?: Record<string, QueryValue>,
): string {
  if (!query) return path;
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== null && value !== undefined && value !== '') {
      params.set(key, String(value));
    }
  }
  const search = params.toString();
  return search ? `${path}?${search}` : path;
}
