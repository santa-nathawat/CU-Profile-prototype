import type { NextRequest } from 'next/server';

export const dynamic = 'force-dynamic';

const REQUEST_HEADERS = [
  'accept',
  'content-length',
  'content-type',
  'cookie',
  'range',
  'user-agent',
];

// Connection-level headers, and ones fetch has already acted on by decoding.
const DROPPED_RESPONSE_HEADERS = new Set([
  'connection',
  'content-encoding',
  'keep-alive',
  'proxy-authenticate',
  'proxy-authorization',
  'set-cookie',
  'te',
  'trailer',
  'transfer-encoding',
  'upgrade',
]);

function errorResponse(status: number, code: string, message: string) {
  return Response.json({ error: { code, message } }, { status });
}

async function forward(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
): Promise<Response> {
  const { path } = await context.params;
  if (path.some((part) => part === '.' || part === '..')) {
    return errorResponse(404, 'not_found', 'Not found.');
  }

  const base = process.env.API_URL ?? 'http://localhost:3001';
  const target = new URL(
    `/api/v1/${path.map(encodeURIComponent).join('/')}${request.nextUrl.search}`,
    base,
  );

  const headers = new Headers();
  for (const name of REQUEST_HEADERS) {
    const value = request.headers.get(name);
    if (value !== null) headers.set(name, value);
  }

  const hasBody = request.method !== 'GET' && request.method !== 'HEAD';

  let upstream: Response;
  try {
    upstream = await fetch(target, {
      method: request.method,
      headers,
      // Streamed so a CV upload is never held in memory here.
      body: hasBody ? request.body : undefined,
      duplex: hasBody ? 'half' : undefined,
      redirect: 'manual',
      cache: 'no-store',
      signal: request.signal,
    } as RequestInit);
  } catch {
    return errorResponse(
      502,
      'api_unreachable',
      "We can't reach the server right now. Please try again in a moment.",
    );
  }

  const responseHeaders = new Headers();
  upstream.headers.forEach((value, name) => {
    if (!DROPPED_RESPONSE_HEADERS.has(name)) responseHeaders.set(name, value);
  });
  // Decoded by fetch, so the original length no longer matches the body.
  if (upstream.headers.has('content-encoding')) {
    responseHeaders.delete('content-length');
  }
  for (const cookie of upstream.headers.getSetCookie()) {
    responseHeaders.append('set-cookie', cookie);
  }

  return new Response(upstream.body, {
    status: upstream.status,
    headers: responseHeaders,
  });
}

export {
  forward as GET,
  forward as HEAD,
  forward as POST,
  forward as PUT,
  forward as PATCH,
  forward as DELETE,
  forward as OPTIONS,
};
