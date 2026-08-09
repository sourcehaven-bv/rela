/** Layout-grid width for a property field (TKT-5V8704).
 *
 * The grid is 12 columns; a field spans all 12 unless the view/form config
 * declares otherwise. `span` therefore travels config -> Go -> view JSON ->
 * here, and this module is the ONLY place it becomes CSS.
 */

/** Column count of the property-list grid. Must match `properties-list.css`. */
export const SPAN_COLUMNS = 12

/**
 * fieldSpanStyle converts an authored span into the custom property
 * `properties-list.css` reads.
 *
 * Returns `undefined` — no inline style at all — for the default case, so the
 * stylesheet's own `var(--field-span, 12)` fallback provides full width. That
 * keeps "no span authored" expressed in exactly one place.
 *
 * Out-of-range and non-integer values fall back to full width rather than
 * emitting a broken `grid-column`. The backend already rejects these at config
 * load with a specific error (dataentryconfig.validateSpan), so this is
 * defence-in-depth for values arriving from a hand-crafted API response or an
 * older server — not the primary diagnostic. Deliberately silent: the server
 * is where an author gets told they made a mistake.
 */
export function fieldSpanStyle(span?: number): Record<string, string> | undefined {
  if (!isValidSpan(span)) return undefined
  return { '--field-span': String(span) }
}

/** isValidSpan reports whether a span is an integer within the grid. */
export function isValidSpan(span?: number): span is number {
  return typeof span === 'number' && Number.isInteger(span) && span >= 1 && span <= SPAN_COLUMNS
}
