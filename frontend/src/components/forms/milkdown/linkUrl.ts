/**
 * Validation and normalization for link targets the user supplies.
 *
 * This is the WRITE-side gate. It runs where a URL enters the document — the
 * link dialog and the paste handler — and nowhere else. In particular it must
 * NOT run when a document is parsed on load: `rawHtmlPassthrough.test.ts` pins
 * that an entity's stored bytes survive a round-trip untouched, and rewriting
 * an author's body on open is a data-integrity bug wearing a security fix's
 * clothes. A hostile URL already in a file stays in the file; it is defanged at
 * every render sink instead (the preset's `toDOM`, DOMPurify in the rendered
 * view). This module closes the INSERT half of that surface.
 *
 * Why not delegate to the preset's `sanitizeLinkHref`. It looks like the
 * obvious reuse, and it is the wrong tool twice over:
 *
 *  1. It normalizes only to DECIDE the scheme and then returns the ORIGINAL
 *     string. Hand it a URL with a zero-width space in the path and it hands
 *     that back unchanged, and a leading zero-width is not refused at all.
 *     Those bytes would serialize straight into the entity file — the exact
 *     thing this gate exists to stop.
 *  2. Its allowlist is wider than ours (it also permits `tel:` and `ftp:`) and
 *     it returns any scheme-less string unchanged, so relative paths pass.
 *
 * It remains the right thing on the READ side, where the preset calls it for
 * us. It is deliberately not imported here (it is also absent from the
 * package's public type surface).
 */

/**
 * Characters a browser discards while resolving a URL's scheme.
 *
 * They are the standard way to smuggle a blocked scheme past a naive check —
 * `java<TAB>script:` resolves to `javascript:`. Covers C0/C1 and space,
 * zero-width characters, the unicode line/paragraph separators and the BOM.
 *
 * Kept as a predicate over code points rather than a regex literal because the
 * ranges read as ranges this way, and the set is copied from the same
 * definition the upstream sanitizer uses.
 */
function isIgnoredChar(code: number): boolean {
  return (
    code <= 0x20 ||
    (code >= 0x7f && code <= 0xa0) ||
    (code >= 0x200b && code <= 0x200d) ||
    code === 0x2028 ||
    code === 0x2029 ||
    code === 0xfeff
  )
}

/** Strips the characters above, so what we inspect is what a browser resolves. */
function stripIgnoredChars(input: string): string {
  let out = ''
  for (const char of input) {
    if (!isIgnoredChar(char.codePointAt(0) ?? 0)) out += char
  }
  return out
}

/**
 * Schemes a link may use.
 *
 * An allowlist, not a blocklist: an unknown scheme is refused rather than
 * permitted, so the set of things that can reach a stored `href` only grows
 * deliberately. Narrower than the preset's, which also allows `tel:` and
 * `ftp:`.
 */
const ALLOWED_PROTOCOLS = new Set(['http:', 'https:', 'mailto:'])

/** A scheme per RFC 3986. Used only to detect that the user wrote one. */
const HAS_SCHEME = /^[a-z][a-z0-9+.-]*:/i

/**
 * A bare host we are willing to read as one, so `example.com` can become
 * `https://example.com`.
 *
 * Requires at least one dot, which is what keeps a relative path or a stray
 * word from being promoted to a host. An optional `:port` is part of the
 * pattern rather than left to the scheme check — see `normalizeLinkUrl` for
 * why the order matters.
 */
const HOST_LABEL = '[a-z0-9](?:[a-z0-9-]*[a-z0-9])?'
const BARE_HOST = new RegExp(
  `^${HOST_LABEL}(?:\\.${HOST_LABEL})+(?::\\d{1,5})?(?:[/?#]|$)`,
  'i'
)

/** Why a URL was refused. Shown to the user, so it names the rule, not the input. */
export type LinkUrlRejection =
  | 'empty'
  | 'protocol-relative'
  | 'not-absolute'
  | 'unparseable'
  | 'blocked-scheme'

export interface LinkUrlAccepted {
  ok: true
  /** The normalized, storable value. Never the raw input. */
  url: string
  /**
   * True when a `mailto:` query string was dropped.
   *
   * Surfaced so the UI can say so: silently changing what the user typed is
   * worse than refusing it.
   */
  strippedParams?: boolean
}

export interface LinkUrlRejected {
  ok: false
  reason: LinkUrlRejection
  /** Human-readable, safe to render: describes the rule, never echoes input. */
  message: string
}

export type LinkUrlResult = LinkUrlAccepted | LinkUrlRejected

const MESSAGES: Record<LinkUrlRejection, string> = {
  empty: 'Enter a link address.',
  'protocol-relative': 'Enter a full address starting with https://.',
  'not-absolute': 'Enter a full web address, for example https://example.com.',
  unparseable: 'That is not a valid web address.',
  'blocked-scheme': 'Only http, https and mailto links are allowed.',
}

function reject(reason: LinkUrlRejection): LinkUrlRejected {
  return { ok: false, reason, message: MESSAGES[reason] }
}

/**
 * Validates a user-supplied link target and returns the value to store.
 *
 * Accepts `http:`, `https:` and `mailto:`. A bare host (`example.com`,
 * `example.com:8080/path`) is promoted to `https://`. Everything else is
 * refused, including relative paths — a link in an entity body is read on
 * several surfaces that do not share this app's base URL, so a relative target
 * has no stable meaning.
 *
 * Nil: rejected — a non-string or empty input returns `{ok: false}`.
 */
export function normalizeLinkUrl(input: string): LinkUrlResult {
  const cleaned = stripIgnoredChars(String(input ?? '')).trim()
  if (!cleaned) return reject('empty')

  // Protocol-relative inherits the page's scheme while reading like a path,
  // so `//evil.com/x` looks local and is not. Refused before anything else
  // gets a chance to treat it as one.
  if (cleaned.startsWith('//')) return reject('protocol-relative')

  // The bare-host test runs BEFORE the scheme test, and the order is
  // load-bearing: `example.com:8080/path` matches HAS_SCHEME as a bogus
  // `example.com:` scheme, so checking that first would refuse a perfectly
  // ordinary paste with a message about blocked schemes.
  let candidate: string
  if (BARE_HOST.test(cleaned)) candidate = `https://${cleaned}`
  else if (HAS_SCHEME.test(cleaned)) candidate = cleaned
  else return reject('not-absolute')

  let parsed: URL
  try {
    parsed = new URL(candidate)
  } catch {
    return reject('unparseable')
  }

  if (!ALLOWED_PROTOCOLS.has(parsed.protocol)) return reject('blocked-scheme')

  // A `mailto:` query is the one allowlisted construct with an attacker-useful
  // side effect: `?bcc=` or `?body=` rides along under innocuous link text, so
  // a reader who clicks sends more than they can see. Nobody hand-authors that
  // in an entity body, and this repo already treats mail header injection as a
  // real threat (internal/mail rejects CR/LF in caller-supplied headers).
  // Dropped rather than refused, because the address itself is fine.
  // The FRAGMENT is stripped too, not just the query. `new URL()` only
  // populates `search` when `?` precedes `#`, so `mailto:a@b.com#x?bcc=evil`
  // parks the whole parameter string in `hash` and sails past a query-only
  // check. `mailto:` has no meaningful fragment, so dropping both closes that
  // and costs nothing.
  if (parsed.protocol === 'mailto:') {
    const hadParams = Boolean(parsed.search || parsed.hash)
    parsed.search = ''
    parsed.hash = ''
    // Assigning '' does not remove a trailing bare `?`/`#` from `href`, so the
    // delimiters are trimmed rather than trusted.
    const url = parsed.href.replace(/[?#]+$/, '')
    return hadParams ? { ok: true, url, strippedParams: true } : { ok: true, url }
  }

  // `parsed.href`, never `candidate`: this is the normalized form, and it is
  // what removes any smuggled characters from the value we store.
  return { ok: true, url: parsed.href }
}
