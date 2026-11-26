import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { resolveIcon, hasIcon, ICONS, NO_ICON } from '@/utils/icons'

/**
 * Render tests for the icon element the sidebar and kanban headers use.
 *
 * WHY THIS FILE EXISTS
 * The registry tests verify `resolveIcon` returns the right component OBJECT.
 * They cannot see whether a template actually binds it, whether the `size`
 * prop survives, or whether CSS then distorts the result — and a real bug
 * shipped through exactly that gap: `.nav-icon { width: 24px }` overrode
 * Lucide's `width` presentation attribute while `height` stayed at 18, so
 * every sidebar icon rendered 24x18. A squashed circle still reads as a
 * circle at 18px, which is why review missed it.
 *
 * Mounting the icon directly (rather than the whole Sidebar, which needs a
 * router, three stores and an API) is enough to pin the contract that broke:
 * it must be an <svg> and it must be square.
 */
describe('sidebar icon element', () => {
  it('renders an SVG, not a text glyph', () => {
    const w = mount(resolveIcon('dashboard'), { props: { size: 18 } })
    expect(w.element.tagName.toLowerCase()).toBe('svg')
    // The emoji this replaced were text nodes; assert we are past that.
    expect(w.text()).toBe('')
  })

  it('emits square width/height attributes for the requested size', () => {
    // Lucide sets these as PRESENTATION attributes, which CSS can override —
    // hence the paired assertion. If a stylesheet sets only `width`, the
    // rendered box stops being square even though both attributes are 18.
    const w = mount(resolveIcon('dashboard'), { props: { size: 18 } })
    expect(w.attributes('width')).toBe('18')
    expect(w.attributes('height')).toBe('18')
  })

  it('uses currentColor so it follows the theme', () => {
    // The entire justification for replacing emoji: they could not inherit
    // the surrounding text colour.
    const w = mount(resolveIcon('dashboard'), { props: { size: 18 } })
    expect(w.attributes('stroke')).toBe('currentColor')
  })

  it('renders the fallback for an unknown name rather than blowing up', () => {
    const w = mount(resolveIcon('no-such-icon'), { props: { size: 18 } })
    expect(w.element.tagName.toLowerCase()).toBe('svg')
  })

  it('renders every registered icon as an SVG', () => {
    // Guards against a registry entry that is not a component at all.
    for (const name of Object.keys(ICONS)) {
      const w = mount(resolveIcon(name), { props: { size: 18 } })
      expect(w.element.tagName.toLowerCase(), `icon ${name}`).toBe('svg')
    }
  })
})

/**
 * Kanban column and swimlane headers gate on hasIcon, exactly as the sidebar
 * does.
 *
 * Mounting KanbanView needs a router, several stores and an API, so this pins
 * the decision the three template sites make rather than the templates
 * themselves — the bug being guarded against is the two surfaces disagreeing
 * about what counts as "no icon", not a markup detail.
 *
 * Before `none` existed, kanban used a bare `v-if="column.icon"`. That is true
 * for the string 'none', so a `none` column would have fallen through to
 * resolveIcon and drawn the FileText fallback: an author asking for no icon
 * would have got a document glyph.
 */
describe('kanban column icon gating', () => {
  it('draws a glyph only when the column asked for one', () => {
    expect(hasIcon('inbox')).toBe(true)
    expect(hasIcon(NO_ICON)).toBe(false)
    expect(hasIcon(undefined)).toBe(false)
  })

  it('would have drawn a fallback glyph under the old bare-truthiness check', () => {
    // Documents precisely why the guard changed: 'none' is a non-empty string,
    // so the previous condition passed it straight through.
    expect(Boolean(NO_ICON)).toBe(true)
    expect(resolveIcon(NO_ICON)).toBe(ICONS.document)
  })
})
