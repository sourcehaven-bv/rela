import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

/**
 * Cascade tests for the property-list grid.
 *
 * WHY THIS FILE EXISTS
 * Three real layout bugs shipped in this feature and were caught by looking at
 * a browser, not by the suite:
 *
 *   1. `.form-fields > *` and `.form-field` both set `grid-column` at equal
 *      specificity, so source order won and every authored span was silently
 *      swallowed.
 *   2. Relation widgets carry their own root class, so they became auto-width
 *      grid items and collapsed the form.
 *   3. Grid items default to `align-items: stretch`, so one tall field made its
 *      row-mates equally tall and left a void under their inputs.
 *
 * All three were CASCADE-RESOLUTION failures, not geometry failures — and jsdom
 * resolves the cascade, including custom-property fallbacks. `fieldSpan.test.ts`
 * covers the helper that PRODUCES `--field-span`; nothing covered the rule that
 * CONSUMES it, which is where the whole design actually lives.
 *
 * What jsdom genuinely cannot do is box geometry: track sizing, wrapping, real
 * pixel widths. Don't add assertions of that kind here — they'd pass vacuously.
 */

// Resolved from the vitest root (frontend/) rather than import.meta.url, which
// is not a file:// URL under the transform pipeline.
const cssPath = resolve(process.cwd(), 'src/styles/properties-list.css')

let styleEl: HTMLStyleElement

beforeAll(() => {
  styleEl = document.createElement('style')
  styleEl.textContent = readFileSync(cssPath, 'utf8')
  document.head.appendChild(styleEl)
})

afterAll(() => styleEl.remove())

/** render builds a list with one item per span and returns the items. */
function render(
  spans: Array<number | undefined>,
  opts: { long?: number[]; compact?: boolean } = {}
) {
  const list = document.createElement('dl')
  list.className = opts.compact ? 'properties-list properties-list--compact' : 'properties-list'
  spans.forEach((span, i) => {
    const item = document.createElement('div')
    item.className = opts.long?.includes(i) ? 'property-item property-long' : 'property-item'
    if (span !== undefined) item.style.setProperty('--field-span', String(span))
    list.appendChild(item)
  })
  document.body.appendChild(list)
  return { list, items: [...list.children] as HTMLElement[] }
}

describe('properties-list grid', () => {
  it('is a 12-column grid that top-aligns its items', () => {
    const { list } = render([])
    const s = getComputedStyle(list)
    expect(s.display).toBe('grid')
    // Regression guard for bug 3: `stretch` makes every field in a row as tall
    // as the tallest, stranding short fields above a void.
    expect(s.alignItems).toBe('start')
  })

  it('gives an unspanned item the full row via the var() fallback', () => {
    // The full-width default lives ONLY in `var(--field-span, 12)`. If someone
    // adds a literal `span 12` elsewhere, this still passes — but the
    // equal-specificity test below is what catches that.
    const { items } = render([undefined])
    expect(getComputedStyle(items[0]).gridColumn).toBe('span 12')
  })

  it('applies an authored span', () => {
    const { items } = render([4, 6, 3])
    expect(items.map((i) => getComputedStyle(i).gridColumn)).toEqual(['span 4', 'span 6', 'span 3'])
  })

  it('lets .property-long override an authored span', () => {
    // Documented precedence: a paragraph squeezed into a third of a row is
    // unreadable whatever the config says.
    const { items } = render([4], { long: [0] })
    expect(getComputedStyle(items[0]).gridColumn).toBe('span 12')
  })

  it('collapses to one column in the compact (side-panel) variant', () => {
    // The rail is a few hundred px wide, so the 12-column model does not apply.
    const { list, items } = render([4], { compact: true })
    expect(getComputedStyle(list).gridTemplateColumns).toBe('minmax(0, 1fr)')
    expect(getComputedStyle(items[0]).gridColumn).toBe('span 1')
  })

  it('does not SHOUT labels', () => {
    // Consolidating three forked copies dropped `text-transform: uppercase`
    // from all three surfaces. That was the intent — labels were uppercase on
    // the detail page and sentence-case in forms — but it was an unannounced
    // change, so pin it as a decision rather than leaving it implicit.
    const { items } = render([undefined])
    const dt = document.createElement('dt')
    items[0].appendChild(dt)
    // jsdom reports '' when no rule sets the property (it does not synthesise
    // the initial value), so assert "not uppercase" rather than a literal —
    // both '' and 'none' mean the labels are no longer shouted.
    expect(getComputedStyle(dt).textTransform).not.toBe('uppercase')
  })
})
