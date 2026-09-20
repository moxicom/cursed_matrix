/**
 * The one place that talks to the server.
 *
 * Nothing here holds a token. The session lives in HttpOnly cookies the page
 * cannot read, so a script that gets injected has nothing to steal; the
 * browser attaches them and the only thing this file adds is the CSRF token,
 * which is deliberately readable so that it can be echoed back.
 */

const BASE = '/api/v1';

/** Set by the server alongside the session, readable so it can be echoed. */
const CSRF_COOKIE = 'cm_csrf';
const CSRF_HEADER = 'X-CSRF-Token';

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS']);

/**
 * A refusal the server explained.
 *
 * `code` is language-independent and `params` carries values for the message
 * template, so the client renders it in whichever language the user reads —
 * the server never sends prose.
 */
export class ApiError extends Error {
  constructor(
    readonly code: string,
    readonly status: number,
    readonly params: Record<string, unknown> = {},
    readonly requestId?: string,
  ) {
    super(`${code} (${status})`);
    this.name = 'ApiError';
  }

  /** True when the session is gone and the user has to sign in again. */
  get unauthenticated(): boolean {
    return this.status === 401;
  }

  /** True when the plan is what stands in the way, not the request. */
  get requiresPayment(): boolean {
    return this.status === 402;
  }
}

function csrfToken(): string | null {
  for (const entry of document.cookie.split(';')) {
    const [name, ...rest] = entry.trim().split('=');
    if (name === CSRF_COOKIE) return rest.join('=');
  }
  return null;
}

interface Envelope {
  error?: { code?: string; params?: Record<string, unknown>; requestId?: string };
}

async function refusal(response: Response): Promise<ApiError> {
  let body: Envelope = {};
  try {
    body = (await response.json()) as Envelope;
  } catch {
    // A proxy or a crash can answer without the envelope; the status is then
    // all there is to go on.
  }

  return new ApiError(
    body.error?.code ?? 'INTERNAL',
    response.status,
    body.error?.params ?? {},
    body.error?.requestId,
  );
}

export interface RequestOptions {
  /** Query parameters; undefined and null entries are left out entirely. */
  query?: Record<string, string | number | boolean | string[] | undefined | null>;
  body?: unknown;
  signal?: AbortSignal;
}

function url(path: string, query: RequestOptions['query']): string {
  if (!query) return BASE + path;

  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    // Absent, not empty: a caller that has nothing to say for a parameter
    // leaves it out, and an empty string it does pass is a value.
    if (value === undefined || value === null) continue;
    // Repeated rather than comma-joined: that is how the contract reads a list.
    if (Array.isArray(value)) value.forEach((entry) => search.append(key, entry));
    else search.append(key, String(value));
  }

  const rendered = search.toString();
  return rendered ? `${BASE}${path}?${rendered}` : BASE + path;
}

async function request<T>(method: string, path: string, options: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {};
  if (options.body !== undefined) headers['Content-Type'] = 'application/json';

  if (!SAFE_METHODS.has(method)) {
    const token = csrfToken();
    // Absent before the first response of a session; the server refuses the
    // request either way, and sending an empty header would say nothing.
    if (token) headers[CSRF_HEADER] = token;
  }

  // Built rather than spread with undefined values: the project forbids an
  // optional property that is present and undefined, and fetch reads the
  // difference.
  const init: RequestInit = {
    method,
    headers,
    // Same origin through the proxy, so the cookies ride along; saying it
    // explicitly keeps the dev server honest too.
    credentials: 'same-origin',
  };
  if (options.body !== undefined) init.body = JSON.stringify(options.body);
  if (options.signal) init.signal = options.signal;

  const response = await fetch(url(path, options.query), init);

  if (!response.ok) throw await refusal(response);
  if (response.status === 204) return undefined as T;

  try {
    return (await response.json()) as T;
  } catch {
    // A proxy answering 200 with a page of HTML is not an outage, and saying
    // so would send the caller looking in the wrong place.
    throw new ApiError('MALFORMED_RESPONSE', response.status);
  }
}

export const api = {
  get: <T>(path: string, options?: RequestOptions) => request<T>('GET', path, options),
  post: <T>(path: string, options?: RequestOptions) => request<T>('POST', path, options),
  patch: <T>(path: string, options?: RequestOptions) => request<T>('PATCH', path, options),
  delete: <T>(path: string, options?: RequestOptions) => request<T>('DELETE', path, options),
};
