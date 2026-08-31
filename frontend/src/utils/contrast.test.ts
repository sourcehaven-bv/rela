import { describe, expect, it } from 'vitest'

import { contrastRatio, flatten, relativeLuminance } from './contrast'

describe('contrast math', () => {
  it('matches the WCAG reference points', () => {
    expect(contrastRatio('#000000', '#ffffff')).toBeCloseTo(21, 5)
    expect(contrastRatio('#ffffff', '#ffffff')).toBeCloseTo(1, 5)
    expect(relativeLuminance('#ffffff')).toBeCloseTo(1, 5)
    expect(relativeLuminance('#000000')).toBeCloseTo(0, 5)
  })

  it('is symmetric', () => {
    expect(contrastRatio('#4772fb', '#ffffff')).toBeCloseTo(contrastRatio('#ffffff', '#4772fb'), 8)
  })
})

/**
 * Pins the gantt view's color pairs to WCAG AA. Theme token values are copied
 * from styles/tokens.css — if the theme changes these, this test tells us the
 * gantt needs a re-check rather than silently drifting below AA.
 *
 * Thresholds: 4.5:1 for normal-size TEXT (1.4.3); 3:1 for meaningful
 * NON-TEXT graphics against adjacent colors (1.4.11).
 */
describe('gantt view WCAG AA pairs', () => {
  const light = { bg: '#ffffff', text: '#191919', muted: '#6b7280', accent: '#4772fb' }
  const dark = { bg: '#0f1f28', text: '#ece9e0', accent: '#6f93ff' }

  it('labels: body text on the row background (both themes)', () => {
    expect(contrastRatio(light.text, light.bg)).toBeGreaterThanOrEqual(4.5)
    expect(contrastRatio(dark.text, dark.bg)).toBeGreaterThanOrEqual(4.5)
  })

  it('secondary text: muted on light background', () => {
    expect(contrastRatio(light.muted, light.bg)).toBeGreaterThanOrEqual(4.5)
  })

  it('truncated flag: amber-800 on the amber chip (was borderline at amber-700)', () => {
    expect(contrastRatio('#92400e', '#fef3c7')).toBeGreaterThanOrEqual(4.5)
    // The rejected value, kept as documentation of why it was changed:
    expect(contrastRatio('#b45309', '#fef3c7')).toBeLessThan(4.6)
  })

  it('accent is NOT used for small text on light surfaces (4.16:1 < 4.5:1)', () => {
    // This pin documents WHY labels moved out of the bars: accent-on-white
    // fails AA for normal-size text. It still passes the 3:1 non-text bar
    // for fills and borders, which is all it is used for now.
    expect(contrastRatio(light.accent, light.bg)).toBeLessThan(4.5)
    expect(contrastRatio(light.accent, light.bg)).toBeGreaterThanOrEqual(3)
  })

  it('non-text: bar fills and borders reach 3:1 against their surface', () => {
    expect(contrastRatio(light.accent, light.bg)).toBeGreaterThanOrEqual(3) // leaf fill, parent border
    expect(contrastRatio(dark.accent, dark.bg)).toBeGreaterThanOrEqual(3)
  })

  it('non-text: breach textures reach 3:1 in both themes', () => {
    // Amber overrun dots (dominant dot color) and red past-commit stripes.
    expect(contrastRatio('#b45309', light.bg)).toBeGreaterThanOrEqual(3)
    expect(contrastRatio('#b45309', dark.bg)).toBeGreaterThanOrEqual(3)
    expect(contrastRatio('#dc2626', light.bg)).toBeGreaterThanOrEqual(3)
    expect(contrastRatio('#dc2626', dark.bg)).toBeGreaterThanOrEqual(3)
  })

  it('non-text: the parent envelope fill stays subtle but its border carries the boundary', () => {
    // The 8% wash is decorative (the border is the meaningful edge); flattening
    // shows the wash alone would NOT pass, which is why the border is solid.
    const wash = flatten('#4772fb', 0.08, '#ffffff')
    expect(contrastRatio(wash, '#ffffff')).toBeLessThan(3)
    expect(contrastRatio('#4772fb', '#ffffff')).toBeGreaterThanOrEqual(3)
  })
})
