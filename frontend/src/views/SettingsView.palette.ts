// Pure helpers for the Settings → Appearance palette UI. Extracted
// from SettingsView.vue so they can be unit-tested without mounting
// the whole component (views/ are excluded from coverage anyway, but
// the save-payload shape is critical correctness logic that's worth
// pinning).

import type { PaletteConfig, PaletteColors, DarkPalette } from '@/api/settings'

export type PaletteMode = 'regular' | 'light-dark'

// The 8 valid role keys, kept as a literal-typed const so iteration
// over them is type-checked. A typo (e.g. 'acent') would be caught
// at compile time rather than silently sent on the wire.
export const PALETTE_ROLE_KEYS = [
  'base',
  'surface',
  'accent',
  'text',
  'success',
  'error',
  'warning',
  'info',
] as const satisfies readonly (keyof PaletteColors)[]

export type PaletteRoleKey = typeof PALETTE_ROLE_KEYS[number]

export interface PaletteEditState {
  mode: PaletteMode
  light: Record<string, string>
  badges: Record<string, string>
  dark: Record<string, string>
  darkBadges: Record<string, string>
}

/**
 * Build the JSON payload sent to PUT /api/v1/_palette from the
 * current editor state.
 *
 * Rules:
 *  - Empty light fields are omitted (server treats missing as
 *    "fall back to default").
 *  - Empty badge entries are omitted; if no badges are set the
 *    `badges` key is omitted entirely.
 *  - Regular mode always sends `dark: false`.
 *  - Light+Dark mode always sends a `dark` object containing only
 *    the dark slots the user actually set. The empty-object case
 *    (`{}`) is intentional — the backend then inherits all dark
 *    slots from the light palette, which is the behavior the user
 *    sees in the UI before clicking Derive.
 */
export function buildPalettePayload(state: PaletteEditState): PaletteConfig {
  const palette: PaletteConfig = {}

  // Iterate ROLE_KEYS instead of Object.entries so a typo in
  // state.light is caught at type-check time and stray keys can't
  // be smuggled into the payload.
  for (const key of PALETTE_ROLE_KEYS) {
    const val = state.light[key]
    if (val) palette[key] = val
  }

  const badges: Record<string, string> = {}
  for (const [key, val] of Object.entries(state.badges)) {
    if (val) badges[key] = val
  }
  if (Object.keys(badges).length > 0) palette.badges = badges

  if (state.mode === 'regular') {
    palette.dark = false
  } else {
    const darkPayload: DarkPalette = {}
    for (const key of PALETTE_ROLE_KEYS) {
      const val = state.dark[key]
      if (val) darkPayload[key] = val
    }
    const darkBadges: Record<string, string> = {}
    for (const [key, val] of Object.entries(state.darkBadges)) {
      if (val) darkBadges[key] = val
    }
    if (Object.keys(darkBadges).length > 0) darkPayload.badges = darkBadges
    palette.dark = darkPayload
  }

  return palette
}

/**
 * Determine the initial editor state from a loaded user PaletteConfig.
 *
 * The user palette is an overlay on top of the project palette. The
 * `resolvedDarkDisabled` parameter carries the *effective* dark state
 * after merging user + project so that:
 *
 * - User explicitly set `dark: false`        → Regular mode
 * - User explicitly set `dark: { ... }`      → Light+Dark mode, dark
 *                                              column pre-filled
 * - User did NOT specify dark, but the
 *   project ships dark and resolves to a
 *   non-disabled palette                     → Light+Dark mode (so a
 *                                              naive Save doesn't
 *                                              shadow the project's
 *                                              dark with `dark: false`)
 * - User did NOT specify dark and the
 *   resolved palette has dark disabled       → Regular mode
 *
 * This addresses the asymmetry where loading the user-only overlay
 * would show Regular mode for a user who never set `dark` even
 * though the project they inherit from has a working dark theme.
 */
export function loadPaletteState(
  p: PaletteConfig | undefined,
  roleKeys: string[],
  resolvedDarkDisabled = true,
): PaletteEditState {
  const state: PaletteEditState = {
    mode: 'regular',
    light: {},
    badges: {},
    dark: {},
    darkBadges: {},
  }

  if (p) {
    for (const role of roleKeys) {
      const val = p[role as keyof PaletteConfig]
      if (typeof val === 'string') state.light[role] = val
    }
    state.badges = { ...(p.badges || {}) }
  }

  if (p?.dark === false) {
    state.mode = 'regular'
  } else if (p?.dark && typeof p.dark === 'object') {
    state.mode = 'light-dark'
    const darkObj = p.dark as DarkPalette
    for (const role of roleKeys) {
      const val = darkObj[role as keyof PaletteColors]
      if (typeof val === 'string') state.dark[role] = val
    }
    state.darkBadges = { ...(darkObj.badges || {}) }
  } else {
    // User did not specify dark. Inherit the project's resolved
    // dark state so the editor doesn't silently downgrade a project
    // that ships with a dark theme to Regular mode.
    state.mode = resolvedDarkDisabled ? 'regular' : 'light-dark'
  }

  return state
}
