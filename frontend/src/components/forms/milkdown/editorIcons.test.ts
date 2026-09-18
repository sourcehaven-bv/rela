import { describe, it, expect } from 'vitest'
import { ICON_PARTS, createIconSvg, ICON_SVG_ATTRS } from './editorIcons'
import { INLINE_COMMANDS, BLOCK_COMMANDS } from './editorCommands'
import { TABLE_COMMANDS } from './tableCommands'

describe('editorIcons', () => {
  it('has a glyph for every command the toolbar can show', () => {
    // A missing glyph is an empty button, which is invisible in review and
    // useless to the user. Asserting the SET rather than any one name is what
    // makes this catch the next command added.
    for (const cmd of [...INLINE_COMMANDS, ...BLOCK_COMMANDS, ...TABLE_COMMANDS]) {
      expect(ICON_PARTS[cmd.id], cmd.id).toBeDefined()
      expect(ICON_PARTS[cmd.id].length, cmd.id).toBeGreaterThan(0)
    }
    // Not a command, but the toolbar draws it alongside them.
    expect(ICON_PARTS.entityRef).toBeDefined()
  })

  it('builds an SVG carrying the shared wrapper attributes', () => {
    const svg = createIconSvg('strong')
    for (const [k, v] of Object.entries(ICON_SVG_ATTRS)) {
      expect(svg.getAttribute(k)).toBe(v)
    }
    expect(svg.childElementCount).toBe(ICON_PARTS.strong.length)
  })

  it('renders numerals as text and shapes as empty elements', () => {
    const h2 = createIconSvg('h2')
    const text = h2.querySelector('text')
    expect(text?.textContent).toBe('2')
    expect(h2.querySelector('path')?.textContent).toBe('')
  })

  it('returns an empty SVG for an unknown name rather than throwing', () => {
    // Matches the Vue component, whose v-else-if chain simply falls through: a
    // missing glyph should leave a blank button, not break the toolbar around it.
    const svg = createIconSvg('no-such-glyph')
    expect(svg.childElementCount).toBe(0)
  })

  it('draws in currentColor only, so one glyph works in both themes', () => {
    // Asserts the FULL property, not just the absence of a contradiction. An
    // earlier version gated both checks on the attribute being present, so a
    // glyph that simply omitted them passed trivially — and omission is the
    // likelier shape for hand-written geometry than a hardcoded hex.
    //
    // A part is either FILLED (carries both `fill: currentColor` and
    // `stroke: none`, opting out of the wrapper's stroke) or INHERITING
    // (carries neither, taking the wrapper's `stroke: currentColor`). Anything
    // else pins a colour that cannot follow the theme.
    for (const [name, parts] of Object.entries(ICON_PARTS)) {
      parts.forEach((part, i) => {
        const where = `${name}[${i}]`
        const { fill, stroke } = part.attrs
        const filled = fill !== undefined || stroke !== undefined
        if (!filled) return
        expect(fill, `${where} must fill with currentColor`).toBe('currentColor')
        expect(stroke, `${where} must opt out of the wrapper stroke`).toBe('none')
      })
    }
  })

  it('never pins a literal colour anywhere in the table', () => {
    // The other half: no part may carry a colour-valued attribute at all, so a
    // future glyph cannot introduce one under a name this file does not check.
    const COLOUR_ATTRS = ['color', 'stop-color', 'flood-color', 'lighting-color']
    for (const [name, parts] of Object.entries(ICON_PARTS)) {
      for (const part of parts) {
        for (const attr of COLOUR_ATTRS) {
          expect(part.attrs[attr], `${name} must not pin ${attr}`).toBeUndefined()
        }
        for (const value of Object.values(part.attrs)) {
          expect(String(value), `${name} must not hardcode a colour`).not.toMatch(/^#|^rgb|^hsl/i)
        }
      }
    }
  })
})
