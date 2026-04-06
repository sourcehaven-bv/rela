import { describe, it, expect } from 'vitest'
import {
  normalizeHex,
  parseHexList,
  parseGPL,
  parsePalette,
  parseRelaPalette,
  hexToHSL,
  assignPalette,
} from './palette'

describe('normalizeHex', () => {
  it('normalizes 6-digit with #', () => {
    expect(normalizeHex('#AABBCC')).toBe('#aabbcc')
  })

  it('normalizes bare 6-digit', () => {
    expect(normalizeHex('ddcf99')).toBe('#ddcf99')
  })

  it('expands 3-digit', () => {
    expect(normalizeHex('#f00')).toBe('#ff0000')
  })

  it('expands bare 3-digit', () => {
    expect(normalizeHex('f0f')).toBe('#ff00ff')
  })
})

describe('parseHexList', () => {
  it('parses one per line', () => {
    const result = parseHexList('ddcf99\ncca87b\nb97a60')
    expect(result).toEqual(['#ddcf99', '#cca87b', '#b97a60'])
  })

  it('parses with # prefix', () => {
    const result = parseHexList('#ff0000\n#00ff00')
    expect(result).toEqual(['#ff0000', '#00ff00'])
  })

  it('parses comma separated', () => {
    const result = parseHexList('#ff0000, #00ff00, #0000ff')
    expect(result).toEqual(['#ff0000', '#00ff00', '#0000ff'])
  })

  it('skips invalid tokens', () => {
    const result = parseHexList('ff0000\nhello\n00ff00')
    expect(result).toEqual(['#ff0000', '#00ff00'])
  })

  it('deduplicates', () => {
    const result = parseHexList('ff0000\nff0000\n00ff00')
    expect(result).toEqual(['#ff0000', '#00ff00'])
  })

  it('returns empty for garbage', () => {
    expect(parseHexList('hello world')).toEqual([])
  })

  it('returns empty for empty string', () => {
    expect(parseHexList('')).toEqual([])
  })
})

describe('parseGPL', () => {
  it('parses GIMP Palette format', () => {
    const gpl = `GIMP Palette
Name: Test
Columns: 4
#
255   0   0\tRed
  0 255   0\tGreen
  0   0 255\tBlue`
    const result = parseGPL(gpl)
    expect(result).toEqual(['#ff0000', '#00ff00', '#0000ff'])
  })

  it('skips comments and headers', () => {
    const gpl = `GIMP Palette
Name: Minimal
# This is a comment
128 128 128`
    const result = parseGPL(gpl)
    expect(result).toEqual(['#808080'])
  })

  it('rejects out-of-range values', () => {
    const gpl = `GIMP Palette
#
256 0 0`
    expect(parseGPL(gpl)).toEqual([])
  })
})

describe('parsePalette', () => {
  it('auto-detects GPL format', () => {
    const gpl = `GIMP Palette
#
255 0 0`
    expect(parsePalette(gpl)).toEqual(['#ff0000'])
  })

  it('auto-detects hex list', () => {
    expect(parsePalette('ff0000\n00ff00')).toEqual(['#ff0000', '#00ff00'])
  })
})

describe('parseRelaPalette', () => {
  it('parses basic palette.yaml', () => {
    const yaml = `base: "#1a1a2e"
surface: "#f8fafc"
accent: "#6366f1"
text: "#1e293b"
success: "#10b981"
error: "#ef4444"
warning: "#f59e0b"
info: "#3b82f6"`
    const result = parseRelaPalette(yaml)
    expect(result.colors.base).toBe('#1a1a2e')
    expect(result.colors.accent).toBe('#6366f1')
    expect(Object.keys(result.colors)).toHaveLength(8)
    expect(result.allColors).toHaveLength(8)
  })

  it('parses badges section', () => {
    const yaml = `accent: "#6366f1"
surface: "#f8fafc"
badges:
  blue: "#3b82f6"
  red: "#ef4444"`
    const result = parseRelaPalette(yaml)
    expect(result.badges.blue).toBe('#3b82f6')
    expect(result.badges.red).toBe('#ef4444')
  })

  it('parses explicit dark section', () => {
    const yaml = `accent: "#6366f1"
surface: "#f8fafc"
dark:
  accent: "#818cf8"
  surface: "#121218"`
    const result = parseRelaPalette(yaml)
    expect(result.dark).toBeDefined()
    expect(result.dark!.accent).toBe('#818cf8')
    expect(result.dark!.surface).toBe('#121218')
  })

  it('handles unquoted hex values', () => {
    const yaml = `accent: #6366f1
surface: #f8fafc`
    const result = parseRelaPalette(yaml)
    expect(result.colors.accent).toBe('#6366f1')
  })

  it('handles inline comments', () => {
    const yaml = `accent: "#6366f1" # primary color
surface: "#f8fafc"`
    const result = parseRelaPalette(yaml)
    expect(result.colors.accent).toBe('#6366f1')
  })

  it('returns undefined dark when not present', () => {
    const yaml = `accent: "#6366f1"
surface: "#f8fafc"`
    const result = parseRelaPalette(yaml)
    expect(result.dark).toBeUndefined()
  })

  it('dark: auto does not create dark overrides', () => {
    const yaml = `accent: "#6366f1"
surface: "#f8fafc"
dark: auto`
    const result = parseRelaPalette(yaml)
    expect(result.dark).toBeUndefined()
  })
})

describe('parsePalette auto-detection', () => {
  it('detects rela YAML', () => {
    const yaml = `base: "#1a1a2e"
surface: "#f8fafc"
accent: "#6366f1"`
    const result = parsePalette(yaml)
    expect(result).toContain('#1a1a2e')
    expect(result).toContain('#6366f1')
  })
})

describe('hexToHSL', () => {
  it('pure red', () => {
    const hsl = hexToHSL('#ff0000')
    expect(hsl.h).toBeCloseTo(0, 1)
    expect(hsl.s).toBeCloseTo(1, 1)
    expect(hsl.l).toBeCloseTo(0.5, 1)
  })

  it('pure green', () => {
    const hsl = hexToHSL('#00ff00')
    expect(hsl.h).toBeCloseTo(1 / 3, 1)
    expect(hsl.s).toBeCloseTo(1, 1)
  })

  it('white', () => {
    const hsl = hexToHSL('#ffffff')
    expect(hsl.l).toBeCloseTo(1, 1)
    expect(hsl.s).toBe(0)
  })

  it('black', () => {
    const hsl = hexToHSL('#000000')
    expect(hsl.l).toBeCloseTo(0, 1)
  })
})

describe('assignPalette', () => {
  it('returns empty for no colors', () => {
    const result = assignPalette([])
    expect(result.colors).toEqual({})
    expect(result.badges).toEqual({})
  })

  it('single color assigned to accent', () => {
    const result = assignPalette(['#6366f1'])
    expect(result.colors.accent).toBe('#6366f1')
    expect(Object.keys(result.colors)).toHaveLength(1)
  })

  it('two colors: darkest=base, lightest=surface', () => {
    const result = assignPalette(['#ffffff', '#000000'])
    expect(result.colors.base).toBe('#000000')
    expect(result.colors.surface).toBe('#ffffff')
  })

  it('assigns base as darkest and surface as lightest', () => {
    const colors = ['#1a1a2e', '#f8fafc', '#6366f1', '#1e293b']
    const result = assignPalette(colors)
    expect(result.colors.base).toBe('#1a1a2e')
    expect(result.colors.surface).toBe('#f8fafc')
  })

  it('assigns green hue to success', () => {
    const colors = [
      '#000000', '#ffffff', '#333333', '#666666', // structural
      '#22c55e', // green → should map to success
      '#ef4444', // red → error
      '#3b82f6', // blue → info
      '#f59e0b', // yellow → warning
    ]
    const result = assignPalette(colors)
    expect(result.colors.success).toBe('#22c55e')
    expect(result.colors.error).toBe('#ef4444')
  })

  it('assigns badge colors by hue with enough colors', () => {
    // 16 colors — enough for all 15 roles
    const colors = [
      '#000000', '#ffffff', '#333333', '#666666',
      '#22c55e', '#ef4444', '#3b82f6', '#f59e0b',
      '#8b5cf6', '#f97316', '#eab308', '#6b7280',
      '#cc3344', '#1e40af', '#15803d', '#c2410c',
    ]
    const result = assignPalette(colors)
    // With 16 colors, all badge slots should be filled
    expect(Object.keys(result.badges).length).toBeGreaterThanOrEqual(5)
    // Gray should be low-saturation
    if (result.badges.gray) {
      const grayHSL = hexToHSL(result.badges.gray)
      expect(grayHSL.s).toBeLessThan(0.3)
    }
  })

  it('no duplicate UI role assignments', () => {
    const colors = ['#111111', '#222222', '#333333', '#444444',
      '#555555', '#666666', '#777777', '#888888']
    const result = assignPalette(colors)
    // UI + semantic roles should be unique (no duplicates within theme colors)
    const themeColors = Object.values(result.colors)
    const unique = new Set(themeColors)
    expect(unique.size).toBe(themeColors.length)
    // Badges may reuse theme colors but should be unique among themselves
    const badgeColors = Object.values(result.badges)
    const uniqueBadges = new Set(badgeColors)
    expect(uniqueBadges.size).toBe(badgeColors.length)
  })

  it('handles red hue wrap-around (350° should match red)', () => {
    // #cc3344 has hue ~352° which should be close to red (0°)
    const colors = [
      '#000000', '#ffffff', '#333333', '#666666',
      '#22c55e', '#cc3344', '#3b82f6', '#f59e0b',
    ]
    const result = assignPalette(colors)
    expect(result.colors.error).toBe('#cc3344')
  })

  it('fading-16 palette assigns reasonable roles', () => {
    const fading16 = [
      '#ddcf99', '#cca87b', '#b97a60', '#9c524e',
      '#774251', '#4b3d44', '#4e5463', '#5b7d73',
      '#8e9f7d', '#645355', '#8c7c79', '#a99c8d',
      '#7d7b62', '#aaa25d', '#846d59', '#a88a5e',
    ]
    const result = assignPalette(fading16)
    // Should have base (darkest), surface (lightest), and several others
    expect(result.colors.base).toBeDefined()
    expect(result.colors.surface).toBeDefined()
    expect(result.colors.accent).toBeDefined()
    // Should have some badge assignments
    expect(Object.keys(result.badges).length).toBeGreaterThan(0)
  })
})
