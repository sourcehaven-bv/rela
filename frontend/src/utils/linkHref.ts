/**
 * Schemes a link may use.
 *
 * An allowlist, not a blocklist: an unknown scheme is refused rather than
 * permitted, so the set of things that can reach an `href` only grows
 * deliberately. Shared by the editor's write-side gate (`normalizeLinkUrl`)
 * and the url widget's read-side check (`linkHref`).
 */
export const LINK_PROTOCOLS: ReadonlySet<string> = new Set(['http:', 'https:', 'mailto:'])

/**
 * The href for a stored value shown as a link, or undefined when it must stay
 * text. This is the READ-side check for the url widget (TKT-0101IO4): the
 * value was written by a form, a script or an import, so nothing upstream
 * vouches for it.
 *
 * It shares the scheme allowlist with the editor's write-side gate but none
 * of its repairs: no bare host is promoted, because `notes.txt` would read as
 * one. The value is parsed by `new URL`, the parser the browser itself uses
 * for an href, so the scheme checked is the scheme that would run. That parser
 * drops leading whitespace and control characters exactly as the browser
 * does, and a zero-width character before the scheme makes the value
 * unparseable. The result is the parsed URL's `href`, never the raw string.
 *
 * A `mailto:` href loses its query and fragment. `?bcc=` or `?body=` would
 * ride along under link text that a narrow cell cuts off, so a reader who
 * clicks would send more than they can see.
 *
 * Nil: accepted — a non-string or empty value returns undefined.
 */
export function linkHref(value: unknown): string | undefined {
  if (typeof value !== 'string' || value.trim() === '') return undefined
  let parsed: URL
  try {
    parsed = new URL(value)
  } catch {
    return undefined
  }
  if (!LINK_PROTOCOLS.has(parsed.protocol)) return undefined
  if (parsed.protocol === 'mailto:') {
    parsed.search = ''
    parsed.hash = ''
    // Assigning '' leaves a trailing bare `?` or `#` in `href`.
    return parsed.href.replace(/[?#]+$/, '')
  }
  return parsed.href
}

/** Whether a link to `href` opens in a new tab: web pages do, a mail link
 * hands off to the mail client and would leave an empty tab behind. */
export function opensInNewTab(href: string): boolean {
  return !href.toLowerCase().startsWith('mailto:')
}
