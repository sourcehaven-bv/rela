/**
 * Values the server writes into the shell before any config can load.
 *
 * A multi-tenant server serves its "setting up" screen from a shell that has
 * no tenant yet, so the app config never loads there. The server names the
 * app and its default language in `<meta>` tags instead (BUG-21N3J4N; see
 * `bootShell` in internal/dataentry/staticshell.go). A shell without them,
 * which is every other shell, reads null.
 */
export type BootMetaName = 'rela-app-name' | 'rela-language'

/** The content of the boot meta `name`, or null when absent or empty. */
export function bootMeta(name: BootMetaName): string | null {
  if (typeof document === 'undefined') return null
  const content = document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)?.content
  return content?.trim() || null
}
