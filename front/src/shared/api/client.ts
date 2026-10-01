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

const REFRESH_PATH = '/auth/refresh';

/**
 * Requests that settle the session themselves. A 401 from one of these is the
 * answer, not a cue to refresh and try again.
 */
const SESSION_PATHS = new Set(['/auth/login', '/auth/register', REFRESH_PATH, '/auth/logout']);

/** One refresh at a time across every tab of this origin. */
const REFRESH_LOCK = 'cm-session-refresh';

let sessionLost: (() => void) | null = null;

/**
 * Registers what happens when the server refuses to refresh the session.
 *
 * The client cannot import the session store — features depend on this file,
 * not the other way round — so the store hands in its own sign-out.
 */
export function onSessionLost(handler: () => void): void {
  sessionLost = handler;
}

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

function send(method: string, path: string, options: RequestOptions): Promise<Response> {
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

  return fetch(url(path, options.query), init);
}

/**
 * Runs one refresh at a time.
 *
 * The server rotates the refresh token on every use and treats a token
 * presented twice as stolen: it revokes every session of the account. Two
 * requests failing together, or two tabs waking up together, would do exactly
 * that to an honest user. The Web Lock spans tabs; where it is missing, a
 * queue at least covers this one.
 */
let queue: Promise<unknown> = Promise.resolve();

async function exclusively<T>(task: () => Promise<T>): Promise<T> {
  if (typeof navigator !== 'undefined' && 'locks' in navigator) {
    return await navigator.locks.request(REFRESH_LOCK, task);
  }
  const run = queue.then(task, task);
  queue = run.catch(() => undefined);
  return run;
}

async function postRefresh(token: string): Promise<Response> {
  return fetch(BASE + REFRESH_PATH, {
    method: 'POST',
    headers: { [CSRF_HEADER]: token },
    credentials: 'same-origin',
  });
}

/**
 * Renews an expired session. True means the caller may try again.
 *
 * `staleToken` is the CSRF token the failed request went out with. The server
 * issues a new one with every pair, so finding a different one once the lock
 * is ours means somebody else has already refreshed, and presenting the
 * refresh token again would be the replay described above.
 *
 * False is the server's verdict that the session is over. Anything else —
 * offline, rate limited, a 5xx — is thrown: it says nothing about the session,
 * and signing the user out for a dropped connection would be a lie.
 */
async function renewSession(staleToken: string | null): Promise<boolean> {
  return exclusively(async () => {
    const current = csrfToken();
    // Nothing to echo: a visitor who never signed in, or cookies long gone.
    if (current === null) return false;
    if (current !== staleToken) return true;

    const response = await postRefresh(current);
    if (response.ok) return true;
    if (response.status === 401 || response.status === 403) return false;
    throw await refusal(response);
  });
}

/**
 * Rotates the pair on purpose, expired or not — after a purchase, so the new
 * plan is in the token. Shares the lock with the automatic refresh.
 */
export async function rotateSession(): Promise<void> {
  const response = await exclusively(() => postRefresh(csrfToken() ?? ''));
  if (!response.ok) throw await refusal(response);
}

async function request<T>(method: string, path: string, options: RequestOptions = {}): Promise<T> {
  const sentWith = csrfToken();
  let response = await send(method, path, options);

  // The access token lasts minutes and the session lasts weeks: a 401 is
  // usually the first, not the second. One refresh, one retry; the user never
  // sees it.
  if (response.status === 401 && !SESSION_PATHS.has(path)) {
    if (await renewSession(sentWith)) response = await send(method, path, options);
    if (response.status === 401) sessionLost?.();
  }

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
