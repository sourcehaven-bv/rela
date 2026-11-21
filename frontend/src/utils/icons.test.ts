import { describe, it, expect } from 'vitest'
import { resolveIcon, isKnownIcon, hasIcon, ICONS, DEFAULT_ICON, iconNames, NO_ICON } from './icons'

describe('resolveIcon', () => {
  it('resolves every name in the registry to its own component', () => {
    for (const name of iconNames()) {
      expect(resolveIcon(name)).toBe(ICONS[name])
    }
  })

  it('falls back to the default for an unknown name rather than throwing', () => {
    // The server rejects unknown names at config load — that is where an
    // author gets told. This path exists so a stale config, an older server,
    // or a hand-crafted response still renders something.
    expect(resolveIcon('no-such-icon')).toBe(DEFAULT_ICON)
    expect(resolveIcon(undefined)).toBe(DEFAULT_ICON)
    expect(resolveIcon(null)).toBe(DEFAULT_ICON)
    expect(resolveIcon('')).toBe(DEFAULT_ICON)
  })

  it('never returns undefined, so callers can bind it without a guard', () => {
    for (const name of ['', 'x', '__proto__', 'constructor', 'toString']) {
      expect(resolveIcon(name)).toBeTruthy()
    }
  })

  it('does not resolve inherited Object properties as icons', () => {
    // A plain `ICONS[name]` lookup would return Object.prototype.toString for
    // `toString`, which is not a component and would break the render. The
    // fallback has to win for prototype keys too.
    expect(resolveIcon('toString')).toBe(DEFAULT_ICON)
    expect(resolveIcon('constructor')).toBe(DEFAULT_ICON)
    expect(isKnownIcon('toString')).toBe(false)
    expect(isKnownIcon('constructor')).toBe(false)
  })
})

describe('icon registry', () => {
  it('is a flat map of lowercase names to components', () => {
    const names = iconNames()
    expect(names.length).toBeGreaterThanOrEqual(120)
    for (const name of names) {
      expect(name).toMatch(/^[a-z][a-z0-9-]*$/)
      expect(ICONS[name]).toBeTruthy()
    }
  })

  it('keeps every name that shipped before the set was expanded', () => {
    // A project may already have authored any of these, so removing or
    // renaming one breaks a config that works today. The Go side pins the same
    // list; both are generated from one table, and this is the SPA half of
    // that contract.
    for (const required of [
      'dashboard', 'list', 'kanban', 'search', 'calendar', 'warning',
      'apps', 'settings', 'document', 'sun', 'moon', 'inbox', 'wrench',
      'done', 'clock', 'status',
    ]) {
      expect(isKnownIcon(required)).toBe(true)
    }
  })

  it('resolves a config string only through a static own-property lookup', () => {
    // The security property: a config value must never be able to name an
    // arbitrary component. Every value is a compile-time import, and the
    // registry is a closed object literal — no dynamic import(), no
    // string-to-component resolution.
    for (const name of iconNames()) {
      const c = ICONS[name]
      expect(['function', 'object']).toContain(typeof c)
    }
  })
})

describe('NO_ICON', () => {
  it('is not a renderable icon', () => {
    // `none` is valid to write in config but names no component. If it ever
    // entered the registry, `icon: none` would draw a glyph — the exact
    // opposite of what the author asked for.
    expect(isKnownIcon(NO_ICON)).toBe(false)
    expect(iconNames()).not.toContain(NO_ICON)
  })

  it('resolves to the fallback if it ever reaches resolveIcon', () => {
    // Callers are supposed to gate on hasIcon first. This is the safety net,
    // and pinning it documents that reaching here is a caller bug, not a
    // supported path.
    expect(resolveIcon(NO_ICON)).toBe(DEFAULT_ICON)
  })
})

describe('hasIcon', () => {
  it('separates "draw nothing" from "derive one" from "draw this"', () => {
    // The single decision point shared by the sidebar and the kanban board.
    // They render the no-glyph case differently — the sidebar reserves the
    // column to keep labels aligned, a kanban header has no column to reserve
    // — but they must agree on WHEN there is no glyph, or they drift apart
    // again the next time a case is added.
    expect(hasIcon('inbox')).toBe(true)
    expect(hasIcon(NO_ICON)).toBe(false)
    expect(hasIcon('')).toBe(false)
    expect(hasIcon(null)).toBe(false)
    expect(hasIcon(undefined)).toBe(false)
  })

  it('does not judge whether the name is known', () => {
    // An unknown name still gets a glyph (the fallback), so hasIcon must say
    // true for it. Conflating "unknown" with "none" would silently turn a
    // typo into a deliberate-looking blank.
    expect(hasIcon('no-such-icon')).toBe(true)
  })
})
