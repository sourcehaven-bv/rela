import { describe, it, expect } from 'vitest'
import { shouldFlipPopover, GRID_COLUMNS } from './popoverFlip'

/** Builds a field list from authored spans; `undefined` means "no span". */
function fields(...spans: (number | undefined)[]) {
  return spans.map((span) => ({ span }))
}

describe('shouldFlipPopover', () => {
  // Regression: flipping aligns the popover to the INDICATOR's right edge, and
  // a full-width field's indicator sits beside its label near the left of the
  // page — so flipping threw the popover off-screen under the sidebar (seen at
  // x=5px against a content area starting at x=264).
  it('does NOT flip a full-width field, whose indicator is at the far left', () => {
    expect(shouldFlipPopover(fields(undefined, undefined), 0)).toBe(false)
    expect(shouldFlipPopover(fields(undefined, undefined), 1)).toBe(false)
    expect(shouldFlipPopover(fields(12), 0)).toBe(false)
  })

  it('flips only the last field of a three-across row', () => {
    const row = fields(4, 4, 4) // exactly fills 12

    expect(shouldFlipPopover(row, 0)).toBe(false)
    expect(shouldFlipPopover(row, 1)).toBe(false)
    expect(shouldFlipPopover(row, 2)).toBe(true)
  })

  it('flips the last field of each row across a wrap', () => {
    // 4+4+4 fills row one; the next 4+4 start row two.
    const twoRows = fields(4, 4, 4, 4, 4)

    expect(shouldFlipPopover(twoRows, 2)).toBe(true) // ends row one
    expect(shouldFlipPopover(twoRows, 3)).toBe(false) // starts row two
    expect(shouldFlipPopover(twoRows, 4)).toBe(true) // last field overall
  })

  it('flips a field whose neighbour cannot fit beside it', () => {
    // 8 + 6 = 14 > 12, so the 6 wraps. Both end up alone on their row, but
    // both are narrower than full width, so their indicators sit far enough
    // right for a leftward popover to stay on screen.
    const row = fields(8, 6)

    expect(shouldFlipPopover(row, 0)).toBe(true)
    expect(shouldFlipPopover(row, 1)).toBe(true)
  })

  it('does not flip a field with room left beside it', () => {
    // 6 + 6 shares a row; the first has a neighbour to its right.
    expect(shouldFlipPopover(fields(6, 6), 0)).toBe(false)
    expect(shouldFlipPopover(fields(6, 6), 1)).toBe(true)
  })

  it('flips the trailing field of a partially-filled row', () => {
    // 4 + 4 leaves 4 columns empty, but nothing follows — grid does not
    // stretch items, so this field is still the rightmost rendered one.
    expect(shouldFlipPopover(fields(4, 4), 1)).toBe(true)
  })

  it('treats an out-of-range or zero span as full width, and so does not flip', () => {
    for (const bad of [0, -3, GRID_COLUMNS + 1, 99]) {
      expect(shouldFlipPopover(fields(bad, 4), 0)).toBe(false)
    }
  })

  it('is safe on missing input and out-of-range indexes', () => {
    expect(shouldFlipPopover(undefined, 0)).toBe(false)
    expect(shouldFlipPopover([], 0)).toBe(false)
    expect(shouldFlipPopover(fields(4, 4), -1)).toBe(false)
    expect(shouldFlipPopover(fields(4, 4), 9)).toBe(false)
  })
})
