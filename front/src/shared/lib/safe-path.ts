/**
 * Redirect targets that came from anywhere but our own route table are not
 * trusted. A "path" that is really an absolute or protocol-relative URL turns a
 * post-login redirect into an open redirect; react-router 6 additionally has a
 * known backslash bypass, so backslashes are rejected outright.
 */
export function isInternalPath(value: unknown): value is string {
  if (typeof value !== 'string' || value === '') return false;
  // must be a site-root path, never "https://evil.com" or "evil.com"
  if (!value.startsWith('/')) return false;
  // "//evil.com" is protocol-relative and leaves the site
  if (value.startsWith('//')) return false;
  // browsers normalise backslashes to slashes: "/\evil.com" escapes too
  if (value.includes('\\')) return false;

  // control characters can smuggle a scheme past naive checks
  for (const character of value) {
    const code = character.codePointAt(0) ?? 0;
    if (code < 0x20 || code === 0x7f) return false;
  }

  return true;
}

export function internalPathOr(value: unknown, fallback: string): string {
  return isInternalPath(value) ? value : fallback;
}
