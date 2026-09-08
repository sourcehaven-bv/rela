/**
 * Grid-row arithmetic for anchored popovers (TKT-FIO205).
 *
 * A field's comment popover is wider than a narrow field, so a left-aligned one
 * on the rightmost field of a row overflows its container. Flipping it to
 * right-aligned is a correctness fix, not a polish item — otherwise the popover
 * is clipped off-screen and its composer is unreachable.
 *
 * Lives here rather than in EntityDetail so it is testable on its own: it is
 * pure arithmetic over authored spans, and the off-by-one cases (a row that
 * exactly fills 12, a field that wraps) are exactly what a unit test should pin.
 */

/** The property grid's track count. Mirrors styles/properties-list.css. */
export const GRID_COLUMNS = 12

/** The subset of a view field this needs: its authored width, if any. */
export interface SpannedField {
  span?: number
}

/**
 * Reports whether the field at `index` should open its popover right-aligned.
 *
 * True when the field is the rightmost on a SHARED grid row. Spans are walked
 * in order to find where rows break, mirroring CSS Grid: an item that does not
 * fit the columns remaining wraps to the next row, leaving the remainder empty
 * (it does NOT stretch to fill).
 *
 * A FULL-WIDTH field is deliberately excluded even though it is trivially
 * "rightmost". Flipping aligns the popover to the INDICATOR's right edge, and a
 * full-width field's indicator sits beside its label near the left of the page
 * — so flipping there throws the popover off the left edge, under the sidebar.
 * Only a field that shares its row with others has its indicator far enough
 * right for a leftward popover to stay on screen.
 */
export function shouldFlipPopover(fields: SpannedField[] | undefined, index: number): boolean {
  if (!fields || index < 0 || index >= fields.length) return false

  let used = 0
  for (let i = 0; i < fields.length; i++) {
    const span = clampSpan(fields[i]?.span)
    // Wrapped onto a new row: this item did not fit in what was left.
    if (used + span > GRID_COLUMNS) used = 0
    used += span

    if (i === index) {
      // Alone on its row: the indicator is at the far left, so a leftward
      // popover would leave the content area. Keep it opening rightward.
      if (span >= GRID_COLUMNS) return false

      // Rightmost when the row is full, or when the NEXT field could not fit
      // beside it — in both cases nothing follows on this row.
      if (used >= GRID_COLUMNS) return true
      const next = clampSpan(fields[i + 1]?.span)
      return i + 1 >= fields.length || used + next > GRID_COLUMNS
    }
  }
  return false
}

/**
 * Normalises an authored span onto the track count.
 *
 * Absent, zero and out-of-range all mean "full width", matching the
 * `var(--field-span, 12)` fallback and utils/fieldSpan.ts's clamp — the server
 * already rejects bad spans at load, so this is defence in depth against a
 * hand-crafted response rather than a routine path.
 */
function clampSpan(span: number | undefined): number {
  if (!span || span < 1 || span > GRID_COLUMNS) return GRID_COLUMNS
  return span
}
