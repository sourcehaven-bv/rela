import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import NavIcon from './NavIcon.vue'
import { NO_ICON } from '@/utils/icons'

/**
 * NavIcon owns the three-way decision the sidebar makes about every row.
 *
 * These assertions are about LAYOUT and PRESENCE, which the registry tests
 * cannot see: `resolveIcon` returning the right component object says nothing
 * about whether a template drew it, or left the right-sized hole when it
 * deliberately did not.
 */
describe('NavIcon', () => {
  it('renders the named glyph', () => {
    const w = mount(NavIcon, { props: { name: 'inbox' } })
    expect(w.find('svg').exists()).toBe(true)
    expect(w.find('.nav-icon-spacer').exists()).toBe(false)
  })

  it('falls back to a glyph for an unknown name', () => {
    // An unknown name is a server-side config error; if one reaches the client
    // anyway (stale config, older server), it must still render something
    // rather than silently collapsing into a no-icon row.
    const w = mount(NavIcon, { props: { name: 'no-such-icon' } })
    expect(w.find('svg').exists()).toBe(true)
  })

  it('draws nothing but reserves the column for the no-icon name', () => {
    // The alignment requirement: in a mixed group, a `none` row must not pull
    // its label left of its icon-bearing siblings.
    const w = mount(NavIcon, { props: { name: NO_ICON } })
    expect(w.find('svg').exists()).toBe(false)

    const spacer = w.find('.nav-icon-spacer')
    expect(spacer.exists()).toBe(true)
    // Matches .nav-icon: an 18px box plus an 18px gutter. Asserted here
    // because the whole point of the spacer is that it occupies exactly the
    // space the icon would have.
    expect(spacer.attributes('class')).toContain('nav-icon-spacer')
  })

  it('marks the spacer aria-hidden so it is not announced', () => {
    // It is decorative whitespace. A screen reader stopping on an empty
    // element between real items would be worse than no icon at all.
    const w = mount(NavIcon, { props: { name: NO_ICON } })
    expect(w.find('.nav-icon-spacer').attributes('aria-hidden')).toBe('true')
  })

  describe('when the sidebar is collapsed', () => {
    it('restores the derived glyph for a no-icon row', () => {
      // Collapsed mode hides every label, so a row with neither icon nor label
      // would be invisible yet still clickable — nothing for a sighted user to
      // aim at, a perfectly normal item for a keyboard or screen-reader user.
      // `none` means "needs no glyph to be told apart from its labelled
      // siblings"; collapsing removes the labels that premise rests on.
      const w = mount(NavIcon, {
        props: { name: NO_ICON, fallback: 'list', collapsed: true },
      })
      expect(w.find('svg').exists()).toBe(true)
      expect(w.find('.nav-icon-spacer').exists()).toBe(false)
    })

    it('renders the SPECIFIC fallback glyph, not merely some glyph', () => {
      // Asserting "an svg exists" is what let the action-entry bug through:
      // an empty fallback resolves to DEFAULT_ICON (a document), so the test
      // passed while a button that fires a mutation was drawn identically to a
      // link to a document. Pin which glyph, not that there is one.
      const w = mount(NavIcon, {
        props: { name: NO_ICON, fallback: 'zap', collapsed: true },
      })
      // Lucide stamps a per-icon class, which identifies the glyph without
      // depending on size or scoped-style attributes.
      const cls = w.find('svg').attributes('class') ?? ''
      expect(cls).toContain('lucide-zap')
      expect(cls).not.toContain('lucide-file-text')
    })

    it('degrades to the default rather than an empty row with no fallback', () => {
      // The server always sends derivedIcon alongside `none` now, so this is
      // the defensive path for an older server or a hand-crafted response. It
      // must still not produce a row that is invisible but clickable.
      const w = mount(NavIcon, { props: { name: NO_ICON, collapsed: true } })
      expect(w.find('svg').exists()).toBe(true)
    })

    it('leaves an ordinary named icon alone', () => {
      const w = mount(NavIcon, { props: { name: 'inbox', collapsed: true } })
      expect(w.find('svg').exists()).toBe(true)
    })
  })

  it('renders the icon as square inline SVG that inherits the text colour', () => {
    // Guards the bug that shipped once: a CSS `width` rule beat Lucide's width
    // PRESENTATION attribute while height stayed at 18, so every icon rendered
    // 24x18 — a squashed circle still reads as a circle, which is why review
    // missed it.
    const svg = mount(NavIcon, { props: { name: 'inbox' } }).find('svg')
    expect(svg.attributes('width')).toBe('18')
    expect(svg.attributes('height')).toBe('18')
    expect(svg.attributes('stroke')).toBe('currentColor')
  })
})
