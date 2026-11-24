/**
 * Values the server writes into the shell before any config can load.
 *
 * A multi-tenant server serves its "setting up" screen from a shell that has
 * no tenant yet, so the app config never loads there. The server names the
 * app and its default language in `<meta>` tags instead (BUG-21N3J4N; see
 * `bootShell` in internal/dataentry/staticshell.go). The pages for a
 * suspended or archived workspace come from the same shell, so their logo,
 * account links, contact text and address, and page texts are there too. A shell without them reads
 * null. The read-only banner's text (`account.read_only_text`) is there too.
 */
export type BootMetaName =
  | 'rela-app-name'
  | 'rela-language'
  | 'rela-languages'
  | 'rela-app-logo'
  | 'rela-sign-out'
  | 'rela-account-url'
  | 'rela-contact'
  | 'rela-contact-email'
  | 'rela-suspended-text'
  | 'rela-archived-text'
  | 'rela-read-only-text'

/** The content of the boot meta `name`, or null when absent or empty. */
export function bootMeta(name: BootMetaName): string | null {
  if (typeof document === 'undefined') return null
  const content = document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)?.content
  return content?.trim() || null
}
