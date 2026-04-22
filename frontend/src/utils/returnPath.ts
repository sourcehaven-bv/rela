/**
 * isSafeReturnPath mirrors the server-side guard of the same name.
 *
 * A `return_to` value is safe only if it is a same-origin path — no
 * scheme, no host — whose path component starts with a single `/`.
 * Rejecting the following classes of payloads:
 *
 *   - Protocol-relative URLs  //evil.com/pwn
 *   - Backslash-tricks        /\evil.com   (browsers normalise \ to /)
 *   - Percent-encoded tricks  /%5Cevil.com, /%2Fevil.com
 *   - Fully-qualified URLs    http://evil.com, javascript:…, mailto:…
 *
 * Returns the normalised path+query+hash on success and the empty string
 * on rejection. Callers should treat `""` as "no redirect target."
 */
/**
 * readReturnTo extracts a safe return_to value from a vue-router query.
 *
 * vue-router gives `route.query` values as `string | string[] | null`
 * (arrays appear when a key is duplicated in the URL). We accept only
 * a single string that passes isSafeReturnPath.
 *
 * Returns the normalised path on success and `null` when the query has
 * no usable return_to.
 */
// Accept the union vue-router exposes on route.query
// (LocationQueryValue | LocationQueryValue[]) without importing its types —
// the literal shape is unknown values that are either strings, null, or
// arrays of those. Anything non-string is rejected.
export function readReturnTo(query: Record<string, unknown>): string | null {
  const raw = query.return_to
  if (typeof raw !== 'string') return null
  const safe = isSafeReturnPath(raw)
  return safe || null
}

export function isSafeReturnPath(s: unknown): string {
  if (typeof s !== 'string' || s === '') return ''
  // Require the literal input to start with a single '/'. Reject
  // protocol-relative ('//...'), backslash-prefixed ('/\\...'), and any
  // percent-encoded separator that would trip a browser to treat the
  // value as off-origin.
  if (!s.startsWith('/')) return ''
  if (
    s.startsWith('//') ||
    s.startsWith('/\\') ||
    s.startsWith('/%5C') ||
    s.startsWith('/%5c') ||
    s.startsWith('/%2F') ||
    s.startsWith('/%2f')
  ) {
    return ''
  }
  // After the prefix-check, URL-parse against a placeholder origin to
  // confirm the result is a pure path + query + hash (no scheme/host).
  let u: URL
  try {
    u = new URL(s, 'https://placeholder.invalid')
  } catch {
    return ''
  }
  if (u.origin !== 'https://placeholder.invalid') return ''
  return u.pathname + u.search + u.hash
}
