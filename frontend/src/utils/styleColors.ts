import type { DotColor, StatusColor } from 'rela-components/types'

/**
 * The bare-hue style classes (`styles:` in data-entry.yaml, served as
 * `badge-<colour>`). Badge.vue reads `badgeClass`; a board column or list
 * section reads `status`, so a column and a badge for the same value show the
 * same colour. Orange and yellow are distinct: orange keeps the library's
 * amber, yellow and teal have their own tokens.
 */
export const STYLE_PALETTE: Record<string, { badgeClass: string; status: DotColor }> = {
  'badge-blue': { badgeClass: 'badge--blue', status: 'blue' },
  'badge-purple': { badgeClass: 'badge--purple', status: 'purple' },
  'badge-green': { badgeClass: 'badge--green', status: 'green' },
  'badge-gray': { badgeClass: 'badge--gray', status: 'grey' },
  'badge-red': { badgeClass: 'badge--red', status: 'red' },
  'badge-orange': { badgeClass: 'badge--orange', status: 'amber' },
  'badge-yellow': { badgeClass: 'badge--yellow', status: '--rl-color-status-yellow' },
  'badge-teal': { badgeClass: 'badge--teal', status: '--rl-color-status-teal' },
}

/**
 * How one style class paints: a badge class for a bare hue, or for a kit shade
 * (`kit-red-700`) and an operator colour (`custom-<name>`) an inline fill with
 * its text colour, both palette custom properties. The server builds the class
 * (`Color.StyleClass` in internal/dataentryconfig/colors.go).
 */
export interface StyleSwatch {
  badgeClass?: string
  badgeStyle?: Record<string, string>
  status: DotColor
}

// The classes the server emits for a shade and an operator colour. The name
// part is already restricted by config validation; the patterns re-check it so
// a hand-crafted response cannot put anything else into a CSS property name.
const FILLED_CLASS =
  /^(kit-(?:grey|red|orange|yellow|green|teal|blue)-(?:50|[1-9]00)|custom-[a-z][a-z0-9-]*)$/

/** The swatch for a style class, if it is one the server can emit. */
export function styleSwatch(cls: string | undefined): StyleSwatch | undefined {
  if (!cls) return undefined
  const named = Object.prototype.hasOwnProperty.call(STYLE_PALETTE, cls)
    ? STYLE_PALETTE[cls]
    : undefined
  if (named) return { badgeClass: named.badgeClass, status: named.status }
  if (!FILLED_CLASS.test(cls)) return undefined
  const token = `--rl-${cls}` as const
  return {
    badgeStyle: { background: `var(${token})`, color: `var(${token}-fg)` },
    status: token,
  }
}

/** The key a value is styled under, normalized the way Badge.vue always has. */
export function styleKey(value: unknown): string {
  return String(value).toLowerCase().replace(/\s/g, '_')
}

/** The style class for `value` in a property's style map, if any. */
export function styleClassFor(
  styles: Record<string, string> | undefined,
  value: unknown
): string | undefined {
  if (!styles || value === undefined || value === null || value === '') return undefined
  return styles[styleKey(value)]
}

/** The dot colour `value` maps to through a property's style map, if any. */
export function styleStatusColor(
  styles: Record<string, string> | undefined,
  value: unknown
): DotColor | undefined {
  return styleSwatch(styleClassFor(styles, value))?.status
}

// The tag tokens each bare-hue class paints with, as Badge.vue's classes do.
const NAMED_FILL: Record<string, string> = {
  'badge-blue': 'blue',
  'badge-purple': 'purple',
  'badge-green': 'green',
  'badge-gray': 'grey',
  'badge-red': 'red',
  'badge-orange': 'amber',
  'badge-yellow': 'yellow',
  'badge-teal': 'teal',
}

/**
 * An inline fill and text colour for a style class, for surfaces that are not
 * a Badge (a dashboard matrix cell). Undefined for a class the server cannot
 * emit, so a cell without one stays neutral.
 */
export function styleFill(cls: string | undefined): Record<string, string> | undefined {
  if (!cls) return undefined
  if (Object.prototype.hasOwnProperty.call(NAMED_FILL, cls)) {
    const tag = NAMED_FILL[cls]
    return { background: `var(--rl-tag-${tag}-bg)`, color: `var(--rl-tag-${tag}-fg)` }
  }
  return styleSwatch(cls)?.badgeStyle
}

/**
 * A solid CSS colour for a style class, for a mark that is not text: a
 * dashboard breakdown bar. It is the colour the value's status dot shows, so
 * a bar and a board column for one value agree. Undefined for a class the
 * server cannot emit, so the caller keeps its default colour.
 */
export function styleBarColor(cls: string | undefined): string | undefined {
  const status = styleSwatch(cls)?.status
  if (!status) return undefined
  if (status.startsWith('--rl-')) return `var(${status})`
  // The dots paint purple with the accent, as the purple badge does.
  return status === 'purple' ? 'var(--rl-color-accent)' : `var(--rl-color-status-${status})`
}

export type { DotColor, StatusColor }
