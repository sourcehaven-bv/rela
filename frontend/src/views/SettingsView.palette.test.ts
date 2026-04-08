import { describe, it, expect } from 'vitest'
import { buildPalettePayload, loadPaletteState, type PaletteEditState } from './SettingsView.palette'
import type { PaletteConfig } from '@/api/settings'

const ROLE_KEYS = ['base', 'surface', 'accent', 'text', 'success', 'error', 'warning', 'info']

/** Build a PaletteEditState with sensible defaults; only specify the
 *  fields that matter for the test. */
function makeState(overrides: Partial<PaletteEditState> = {}): PaletteEditState {
  return {
    mode: 'regular',
    light: {},
    badges: {},
    dark: {},
    darkBadges: {},
    ...overrides,
  }
}

describe('buildPalettePayload', () => {
  describe('regular mode', () => {
    it('always sends dark: false', () => {
      const payload = buildPalettePayload(makeState({ light: { accent: '#ffcd75' } }))
      expect(payload.dark).toBe(false)
    })

    it('omits empty light fields', () => {
      const payload = buildPalettePayload(makeState({ light: { accent: '#ffcd75', surface: '' } }))
      expect(payload.accent).toBe('#ffcd75')
      expect('surface' in payload).toBe(false)
    })

    it('omits empty badges', () => {
      const payload = buildPalettePayload(makeState({ badges: { blue: '#41a6f6', red: '' } }))
      expect(payload.badges).toEqual({ blue: '#41a6f6' })
    })

    it('omits the badges key entirely when no badges are set', () => {
      const payload = buildPalettePayload(makeState({ light: { accent: '#ffcd75' } }))
      expect('badges' in payload).toBe(false)
    })

    it('ignores any dark values that may still be in state', () => {
      // After mode toggle, dark in-memory state may still be populated.
      // Regular-mode save must not leak it.
      const payload = buildPalettePayload(makeState({
        light: { accent: '#ffcd75' },
        dark: { accent: '#abc' },
        darkBadges: { blue: '#5fa9ff' },
      }))
      expect(payload.dark).toBe(false)
    })
  })

  describe('light-dark mode', () => {
    it('sends an empty dark object when no dark slots are set', () => {
      const payload = buildPalettePayload(makeState({
        mode: 'light-dark',
        light: { accent: '#ffcd75' },
      }))
      expect(payload.dark).toEqual({})
    })

    it('sends only the dark slots that are non-empty', () => {
      const payload = buildPalettePayload(makeState({
        mode: 'light-dark',
        light: { accent: '#ffcd75' },
        dark: { accent: '#9294f5', surface: '' },
      }))
      expect(payload.dark).toEqual({ accent: '#9294f5' })
    })

    it('sends a fully-populated dark object after a Derive', () => {
      const fullDark = {
        base: '#11111e', surface: '#0c141d', accent: '#9294f5', text: '#ccd6e5',
        success: '#14e8a2', error: '#f37373', warning: '#f7b13c', info: '#6ca1f8',
      }
      const payload = buildPalettePayload(makeState({
        mode: 'light-dark',
        light: { accent: '#ffcd75' },
        dark: fullDark,
      }))
      expect(payload.dark).toEqual(fullDark)
    })

    it('round-trips dark badges into payload.dark.badges', () => {
      const payload = buildPalettePayload(makeState({
        mode: 'light-dark',
        light: { accent: '#ffcd75' },
        badges: { blue: '#41a6f6', red: '#b13e53' },
        dark: { accent: '#9294f5' },
        darkBadges: { blue: '#5fa9ff' },
      }))
      expect(payload.dark).toEqual({
        accent: '#9294f5',
        badges: { blue: '#5fa9ff' },
      })
    })

    it('omits dark.badges when all dark badge slots are empty', () => {
      const payload = buildPalettePayload(makeState({
        mode: 'light-dark',
        light: { accent: '#ffcd75' },
        dark: { accent: '#9294f5' },
      }))
      expect(payload.dark).toEqual({ accent: '#9294f5' })
    })
  })
})

describe('loadPaletteState', () => {
  it('returns regular defaults when palette is undefined', () => {
    const state = loadPaletteState(undefined, ROLE_KEYS)
    expect(state.mode).toBe('regular')
    expect(state.light).toEqual({})
    expect(state.dark).toEqual({})
    expect(state.badges).toEqual({})
  })

  it('selects regular mode for dark === false', () => {
    const p: PaletteConfig = { accent: '#ffcd75', dark: false }
    const state = loadPaletteState(p, ROLE_KEYS)
    expect(state.mode).toBe('regular')
    expect(state.light).toEqual({ accent: '#ffcd75' })
    expect(state.dark).toEqual({})
  })

  it('selects regular mode when dark is omitted', () => {
    const p: PaletteConfig = { accent: '#ffcd75' }
    const state = loadPaletteState(p, ROLE_KEYS)
    expect(state.mode).toBe('regular')
  })

  it('selects light-dark mode and pre-fills dark column for an explicit object', () => {
    const p: PaletteConfig = {
      accent: '#ffcd75',
      dark: { accent: '#9294f5', surface: '#0c141d' },
    }
    const state = loadPaletteState(p, ROLE_KEYS)
    expect(state.mode).toBe('light-dark')
    expect(state.dark.accent).toBe('#9294f5')
    expect(state.dark.surface).toBe('#0c141d')
    // Unset dark slots are absent (not empty strings).
    expect('base' in state.dark).toBe(false)
  })

  it('round-trips a partial dark override through save then load', () => {
    const initial: PaletteConfig = {
      accent: '#ffcd75',
      surface: '#f4f4f4',
      badges: { blue: '#41a6f6' },
      dark: { accent: '#9294f5' },
    }
    const state = loadPaletteState(initial, ROLE_KEYS)
    const payload = buildPalettePayload(state)
    expect(payload.accent).toBe('#ffcd75')
    expect(payload.surface).toBe('#f4f4f4')
    expect(payload.badges).toEqual({ blue: '#41a6f6' })
    expect(payload.dark).toEqual({ accent: '#9294f5' })
  })

  it('round-trips a fully-derived dark palette', () => {
    const fullDark = {
      base: '#11111e', surface: '#0c141d', accent: '#9294f5', text: '#ccd6e5',
      success: '#14e8a2', error: '#f37373', warning: '#f7b13c', info: '#6ca1f8',
    }
    const initial: PaletteConfig = {
      accent: '#ffcd75',
      dark: fullDark,
    }
    const state = loadPaletteState(initial, ROLE_KEYS)
    expect(state.mode).toBe('light-dark')
    const payload = buildPalettePayload(state)
    expect(payload.dark).toEqual(fullDark)
  })

  it('round-trips dark: false', () => {
    const initial: PaletteConfig = { accent: '#ffcd75', dark: false }
    const state = loadPaletteState(initial, ROLE_KEYS)
    const payload = buildPalettePayload(state)
    expect(payload.dark).toBe(false)
  })

  it('loads explicit dark badges into state.darkBadges', () => {
    const initial: PaletteConfig = {
      accent: '#ffcd75',
      badges: { blue: '#41a6f6' },
      dark: {
        accent: '#9294f5',
        badges: { blue: '#5fa9ff' },
      },
    }
    const state = loadPaletteState(initial, ROLE_KEYS)
    expect(state.mode).toBe('light-dark')
    expect(state.darkBadges).toEqual({ blue: '#5fa9ff' })
    // Round-trips back to the same shape.
    const payload = buildPalettePayload(state)
    expect(payload.dark).toEqual({ accent: '#9294f5', badges: { blue: '#5fa9ff' } })
  })

  describe('user-overrides-project asymmetry (RR-R73K)', () => {
    it('user with no dark field but project ships dark → Light+Dark', () => {
      // The user has no `palette.yaml` (or has one without `dark`).
      // The project ships a dark theme. Without inheriting the
      // resolved state, the editor would show Regular mode and a
      // naive Save would shadow the project's dark with `dark: false`.
      const userOverlay: PaletteConfig = { accent: '#ffcd75' }
      const state = loadPaletteState(userOverlay, ROLE_KEYS, /* resolvedDarkDisabled */ false)
      expect(state.mode).toBe('light-dark')
    })

    it('user with no dark field and project also has dark disabled → Regular', () => {
      const userOverlay: PaletteConfig = { accent: '#ffcd75' }
      const state = loadPaletteState(userOverlay, ROLE_KEYS, /* resolvedDarkDisabled */ true)
      expect(state.mode).toBe('regular')
    })

    it('user explicit dark: false wins over project dark', () => {
      const userOverlay: PaletteConfig = { accent: '#ffcd75', dark: false }
      const state = loadPaletteState(userOverlay, ROLE_KEYS, /* resolvedDarkDisabled */ false)
      expect(state.mode).toBe('regular')
    })

    it('user undefined palette and project ships dark → Light+Dark', () => {
      const state = loadPaletteState(undefined, ROLE_KEYS, /* resolvedDarkDisabled */ false)
      expect(state.mode).toBe('light-dark')
    })
  })
})
