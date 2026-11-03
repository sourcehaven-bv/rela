/**
 * An `external_ref` value (TKT-SM20FG): the id of an entity's counterpart in
 * another system, plus an optional link to it. Only scripts, migrations and
 * imports write one, so the SPA renders it and never sends it back.
 */
export interface ExternalRef {
  id: string
  url?: string
}

/**
 * Reads an external ref from a stored value. A table cell carries the
 * server's string form, which is the id alone. An empty object is no value.
 */
export function parseExternalRef(value: unknown): ExternalRef | undefined {
  if (typeof value === 'string') return value ? { id: value } : undefined
  if (Array.isArray(value)) return parseExternalRef(value[0])
  if (value === null || typeof value !== 'object') return undefined
  const v = value as Record<string, unknown>
  if (typeof v.id !== 'string' || v.id === '') return undefined
  return typeof v.url === 'string' && v.url !== '' ? { id: v.id, url: v.url } : { id: v.id }
}

/**
 * The href for a ref's link, or undefined. Only http and https become a
 * link: the server validates the scheme on write, and this check keeps a
 * value written another way from turning into a javascript: link.
 */
export function externalRefHref(ref: ExternalRef | undefined): string | undefined {
  if (!ref?.url) return undefined
  try {
    const u = new URL(ref.url)
    return u.protocol === 'http:' || u.protocol === 'https:' ? u.href : undefined
  } catch {
    return undefined
  }
}
