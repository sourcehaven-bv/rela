import { describe, it, expect } from 'vitest'
import { resolveIcon, isKnownIcon, ICONS, DEFAULT_ICON, iconNames } from './icons'

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
  it('exposes the names the config allowlist mirrors', () => {
    // The Go side (dataentryconfig.ValidIconNames) is pinned to this list by
    // TestIconAllowlistMatchesFrontend. This assertion only guards the shape
    // that test parses — a registry that stopped being a flat name->component
    // map would break it silently.
    const names = iconNames()
    expect(names.length).toBeGreaterThan(0)
    for (const name of names) {
      expect(name).toMatch(/^[a-z][a-z0-9-]*$/)
    }
  })

  it('includes the names the sidebar and prototype config depend on', () => {
    for (const required of [
      'dashboard',
      'list',
      'kanban',
      'search',
      'warning',
      'apps',
      'settings',
      'sun',
      'moon',
      'inbox',
      'wrench',
      'done',
    ]) {
      expect(isKnownIcon(required)).toBe(true)
    }
  })
})
