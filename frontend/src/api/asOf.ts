/**
 * The instant the app reads at, when the reader is looking at a past state.
 *
 * The page names it in `?as_of=` (see `useAsOf`), and `App.vue` copies it here
 * so the API client can apply it to every read without each caller threading
 * it through. That is unlike the world, which callers pass one by one: an
 * instant applies to the whole page, and a read that missed it would mix
 * today's rows into a past state without anyone noticing.
 *
 * Two rules follow from the server's contract (`internal/dataentry/asof.go`):
 *
 * - Every read of the data carries it (see `carriesAsOf`). The server
 *   answers it at the instant or refuses it; it never answers for today.
 *   The schema, config, sidebar and similar routes describe the app, not the
 *   data, so they are read as they are now.
 * - Nothing is written while an instant is set. The server refuses a write
 *   that names an instant, but a write without one would change today's
 *   data from a page showing the past. The client refuses those itself.
 */

// An RFC 3339 instant with an explicit zone, the only form the server takes.
const INSTANT = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:\d{2})$/

/**
 * The instant named by a raw `?as_of=` value, or undefined when it is absent,
 * repeated or not an instant the server would take. A malformed value would
 * be a 400 on every read, so the page reads the present instead.
 */
export function parseAsOf(raw: unknown): string | undefined {
  if (typeof raw !== 'string' || !INSTANT.test(raw)) return undefined
  return Number.isNaN(Date.parse(raw)) ? undefined : raw
}

// Every change of instant reloads the page (see useAsOf), so the URL the
// page loaded with is the instant for the page's whole life.
let current: string | undefined = parseAsOf(
  typeof window === 'undefined' ? null : new URLSearchParams(window.location.search).get('as_of')
)

/** The instant reads are bound to, or undefined for the current state. */
export function currentAsOf(): string | undefined {
  return current
}

/** Binds reads to `instant` (an RFC 3339 timestamp), or clears it. */
export function setCurrentAsOf(instant: string | undefined): void {
  current = instant || undefined
}

/**
 * Routes that describe the app or the present moment rather than the data:
 * the schema and configuration, the sidebar, the signed-in user, settings,
 * live collaboration (presence, comments, channels, the inbox) and history,
 * which is already about time. They are read as they are now.
 */
const PRESENT_ROUTES = new Set([
  '_schema',
  '_config',
  '_sidebar',
  '_me',
  '_dashboard',
  '_nav_items',
  '_nav_status',
  '_commands',
  '_flows',
  '_transforms',
  '_templates',
  '_git',
  '_remotes',
  '_settings',
  '_theme',
  '_palette',
  '_events',
  '_conflicts',
  '_inbox',
  '_channels',
  '_configure',
  '_next_action',
  '_task_item_conversions',
])
const PRESENT_SUBRESOURCES = new Set([
  '_presence',
  '_typing',
  '_comments',
  '_history',
  '_relation_history',
  '_lifecycle',
  '_conversions',
  '_copies',
])

/**
 * Reports whether a GET of `path` (relative to `/api/v1`) carries the
 * instant. Every read of the data does, so that the server either answers it
 * at the instant or refuses it with `as_of_unsupported`; a read it silently
 * answered for today would mix the present into the past. Only the routes
 * above are left out.
 */
export function carriesAsOf(path: string): boolean {
  const trimmed = path.split('?')[0].replace(/^\/+|\/+$/g, '')
  if (trimmed === '') return false
  const parts = trimmed.split('/')
  if (parts.length === 1 && PRESENT_ROUTES.has(parts[0])) return false
  if (parts[0] === '_schema' || parts[0] === '_channels' || parts[0] === '_flows') return false
  return !parts.some((p) => PRESENT_SUBRESOURCES.has(p))
}
