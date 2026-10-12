import type { StatusColor } from 'rela-components/types'

/**
 * The one table of app style classes (`styles:` in data-entry.yaml, served as
 * `badge-<colour>`). Badge.vue reads `badgeClass`; a relation-backed board
 * column or list section reads `status`, so a column and a badge for the same
 * value show the same colour.
 *
 * Orange and yellow share the amber status token, as their badges already do.
 */
export const STYLE_PALETTE: Record<string, { badgeClass: string; status: StatusColor }> = {
  'badge-blue': { badgeClass: 'badge--blue', status: 'blue' },
  'badge-purple': { badgeClass: 'badge--purple', status: 'purple' },
  'badge-green': { badgeClass: 'badge--green', status: 'green' },
  'badge-gray': { badgeClass: 'badge--gray', status: 'grey' },
  'badge-red': { badgeClass: 'badge--red', status: 'red' },
  'badge-orange': { badgeClass: 'badge--orange', status: 'amber' },
  'badge-yellow': { badgeClass: 'badge--yellow', status: 'amber' },
}

/** The key a value is styled under, normalized the way Badge.vue always has. */
export function styleKey(value: unknown): string {
  return String(value).toLowerCase().replace(/\s/g, '_')
}

/** The style class for `value` in a property's style map, if any. */
export function styleClassFor(
  styles: Record<string, string> | undefined,
  value: unknown,
): string | undefined {
  if (!styles || value === undefined || value === null || value === '') return undefined
  return styles[styleKey(value)]
}

/** The status colour `value` maps to through a property's style map, if any. */
export function styleStatusColor(
  styles: Record<string, string> | undefined,
  value: unknown,
): StatusColor | undefined {
  const cls = styleClassFor(styles, value)
  return cls ? STYLE_PALETTE[cls]?.status : undefined
}
