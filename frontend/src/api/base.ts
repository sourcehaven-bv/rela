/**
 * URL base for a project served under a prefix.
 *
 * The desktop shell can mount several projects at once, each at `/p/<id>/`,
 * and tells the SPA which one this page belongs to with
 * `<meta name="rela-base" content="/p/<id>/">`. Served at the root — the
 * server, and the desktop's active project — there is no meta tag and the
 * base is `/`, so every URL is unchanged.
 *
 * This is read at runtime rather than baked in with Vite's `base`, because
 * `import.meta.env.BASE_URL` is substituted at build time: one bundle could
 * then only ever serve one prefix, and the bundle is embedded in the Go
 * binary.
 */

/** Normalises a base to exactly one leading and one trailing slash. */
function normalise(raw: string): string {
  const trimmed = raw.trim()
  if (trimmed === '' || trimmed === '/') return '/'
  const withLead = trimmed.startsWith('/') ? trimmed : '/' + trimmed
  return withLead.endsWith('/') ? withLead : withLead + '/'
}

// Read once at module init: the tag is server-rendered into the shell and
// cannot change without a page load.
const base: string = normalise(
  document.querySelector<HTMLMetaElement>('meta[name="rela-base"]')?.content ?? '/'
)

/**
 * relaBase returns the SPA's base path, always slash-terminated ("/" when
 * unprefixed). Pass to `createWebHistory` so vue-router prepends it on
 * navigation and strips it from `route.path`.
 */
export function relaBase(): string {
  return base
}

/**
 * apiUrl prefixes a root-absolute path with the project base.
 *
 * Takes a root-absolute path (`/api/v1/_settings`, `/api/command/x`) rather
 * than one relative to `/api/v1`, so the endpoints outside that root — the
 * command, open-file and help routes — go through the same function. A helper
 * that assumed `/api/v1` would silently miss them.
 */
export function apiUrl(path: string): string {
  if (base === '/') return path
  return path.startsWith('/') ? base + path.slice(1) : base + path
}
