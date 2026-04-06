// Palette import: parsing and smart color assignment.

const HEX_RE = /^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/

interface HSL {
  h: number // [0,1]
  s: number // [0,1]
  l: number // [0,1]
}

// --- Parsing ---

/** Normalize a hex color to #rrggbb format. */
export function normalizeHex(raw: string): string {
  let hex = raw.trim().replace(/^#/, '')
  if (hex.length === 3) {
    hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2]
  }
  return '#' + hex.toLowerCase()
}

/** Parse a hex list (one per line, or comma/space separated). Accepts bare hex and #-prefixed. */
export function parseHexList(text: string): string[] {
  const colors: string[] = []
  const tokens = text.split(/[\n,\s]+/).map((t) => t.trim()).filter(Boolean)
  for (const token of tokens) {
    if (HEX_RE.test(token)) {
      colors.push(normalizeHex(token))
    }
  }
  return dedupe(colors)
}

/** Parse a GIMP Palette (.gpl) format string. */
export function parseGPL(text: string): string[] {
  const colors: string[] = []
  const lines = text.split('\n')
  for (const line of lines) {
    const trimmed = line.trim()
    // Skip headers and comments
    if (!trimmed || trimmed.startsWith('GIMP Palette') || trimmed.startsWith('Name:') ||
        trimmed.startsWith('Columns:') || trimmed.startsWith('#')) {
      continue
    }
    // Parse "R G B [name]" — at least 3 numbers
    const match = trimmed.match(/^\s*(\d{1,3})\s+(\d{1,3})\s+(\d{1,3})/)
    if (match) {
      const r = parseInt(match[1], 10)
      const g = parseInt(match[2], 10)
      const b = parseInt(match[3], 10)
      if (r <= 255 && g <= 255 && b <= 255) {
        colors.push(rgbToHex(r, g, b))
      }
    }
  }
  return dedupe(colors)
}

/** Result of parsing a rela palette.yaml file. */
export interface RelaPaletteResult {
  colors: Record<string, string>
  badges: Record<string, string>
  dark?: Record<string, string>
  allColors: string[] // flat list for swatch display
}

/** Detect if text is a rela palette YAML file. */
function isRelaPalette(text: string): boolean {
  // Check for rela palette keys (at least 2 of the 8 role names at root level)
  const roleKeys = ['base:', 'surface:', 'accent:', 'text:', 'success:', 'error:', 'warning:', 'info:']
  let found = 0
  for (const key of roleKeys) {
    if (text.includes(key)) found++
  }
  return found >= 2
}

// Regex to extract hex color from a YAML line like: key: "#aabbcc" or key: #aabbcc
const YAML_HEX_RE = /^\s{0,4}(\w[\w-]*):\s*"?#?([0-9a-fA-F]{6})"?\s*(?:#.*)?$/

/** Parse a rela palette.yaml file into structured colors. */
export function parseRelaPalette(text: string): RelaPaletteResult {
  const colors: Record<string, string> = {}
  const badges: Record<string, string> = {}
  const dark: Record<string, string> = {}
  const allColors: string[] = []

  const roleKeys = new Set(['base', 'surface', 'accent', 'text', 'success', 'error', 'warning', 'info'])
  const badgeKeys = new Set(['blue', 'purple', 'green', 'gray', 'red', 'orange', 'yellow'])

  let section: 'root' | 'badges' | 'dark' = 'root'

  for (const line of text.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) continue

    // Detect section changes (non-indented keys ending with :)
    if (trimmed === 'badges:') { section = 'badges'; continue }
    if (trimmed === 'dark:' || trimmed.startsWith('dark:')) {
      // dark: auto / dark: false / dark: (object follows)
      if (trimmed === 'dark: auto' || trimmed === 'dark: false' || trimmed === 'dark:') {
        section = 'dark'
        continue
      }
    }
    // Any non-indented key resets to root
    if (!line.startsWith(' ') && !line.startsWith('\t') && trimmed.includes(':')) {
      if (section === 'badges' || section === 'dark') {
        // Only reset if it's a known root key
        const key = trimmed.split(':')[0].trim()
        if (roleKeys.has(key)) section = 'root'
      }
    }

    const match = trimmed.match(YAML_HEX_RE)
    if (!match) continue

    const key = match[1]
    const hex = '#' + match[2].toLowerCase()
    allColors.push(hex)

    if (section === 'badges' && badgeKeys.has(key)) {
      badges[key] = hex
    } else if (section === 'dark' && roleKeys.has(key)) {
      dark[key] = hex
    } else if (section === 'root' && roleKeys.has(key)) {
      colors[key] = hex
    }
  }

  return { colors, badges, dark: Object.keys(dark).length > 0 ? dark : undefined, allColors: dedupe(allColors) }
}

/** Auto-detect format and parse palette text. Returns flat color array for swatches. */
export function parsePalette(text: string): string[] {
  if (text.trim().startsWith('GIMP Palette')) {
    return parseGPL(text)
  }
  if (isRelaPalette(text)) {
    return parseRelaPalette(text).allColors
  }
  return parseHexList(text)
}

function dedupe(colors: string[]): string[] {
  return [...new Set(colors)]
}

function rgbToHex(r: number, g: number, b: number): string {
  return '#' + [r, g, b].map((c) => c.toString(16).padStart(2, '0')).join('')
}

// --- HSL Color Math ---

export function hexToHSL(hex: string): HSL {
  const normalized = normalizeHex(hex).replace('#', '')
  const r = parseInt(normalized.slice(0, 2), 16) / 255
  const g = parseInt(normalized.slice(2, 4), 16) / 255
  const b = parseInt(normalized.slice(4, 6), 16) / 255

  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const l = (max + min) / 2

  if (max === min) return { h: 0, s: 0, l }

  const d = max - min
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min)

  let h = 0
  if (max === r) { h = (g - b) / d + (g < b ? 6 : 0) }
  else if (max === g) { h = (b - r) / d + 2 }
  else { h = (r - g) / d + 4 }
  h /= 6

  return { h, s, l }
}

/** Circular hue distance in [0, 0.5] range. */
function hueDist(h1: number, h2: number): number {
  const d = Math.abs(h1 - h2)
  return Math.min(d, 1 - d)
}

/** Weighted color distance for hue matching. */
function colorDist(c: HSL, target: HSL): number {
  return hueDist(c.h, target.h) * 3 + Math.abs(c.s - target.s) + Math.abs(c.l - target.l) * 0.5
}

// --- Smart Assignment ---

export interface PaletteAssignment {
  colors: Record<string, string>   // role key → hex
  badges: Record<string, string>   // badge name → hex
}

// Target hues for semantic roles (normalized [0,1])
const SEMANTIC_TARGETS: { key: string; h: number; s: number; l: number }[] = [
  { key: 'success', h: 145 / 360, s: 0.6, l: 0.45 },
  { key: 'error',   h: 0 / 360,   s: 0.8, l: 0.55 },
  { key: 'warning', h: 38 / 360,  s: 0.9, l: 0.52 },
  { key: 'info',    h: 217 / 360, s: 0.9, l: 0.60 },
]

// Target hues for badge colors
const BADGE_TARGETS: { key: string; h: number; s: number; l: number; isGray?: boolean }[] = [
  { key: 'red',    h: 0 / 360,   s: 0.8, l: 0.55 },
  { key: 'orange', h: 25 / 360,  s: 0.9, l: 0.53 },
  { key: 'yellow', h: 48 / 360,  s: 0.8, l: 0.47 },
  { key: 'green',  h: 142 / 360, s: 0.7, l: 0.45 },
  { key: 'blue',   h: 217 / 360, s: 0.9, l: 0.60 },
  { key: 'purple', h: 259 / 360, s: 0.9, l: 0.55 },
  { key: 'gray',   h: 0,         s: 0,   l: 0.5, isGray: true },
]

/** Assign imported colors to palette roles using heuristic matching. */
export function assignPalette(hexColors: string[]): PaletteAssignment {
  if (hexColors.length === 0) return { colors: {}, badges: {} }

  const result: PaletteAssignment = { colors: {}, badges: {} }
  const hsls = hexColors.map((c) => ({ hex: c, hsl: hexToHSL(c) }))
  const used = new Set<string>()

  // Step 1: Assign UI structural roles by lightness
  const byLightness = [...hsls].sort((a, b) => a.hsl.l - b.hsl.l)

  if (hexColors.length === 1) {
    result.colors.accent = hexColors[0]
    used.add(hexColors[0])
    return result
  }

  // base = darkest, surface = lightest
  result.colors.base = byLightness[0].hex
  used.add(byLightness[0].hex)

  result.colors.surface = byLightness[byLightness.length - 1].hex
  used.add(byLightness[byLightness.length - 1].hex)

  if (hexColors.length >= 3) {
    // text = second darkest (skip if same as base)
    for (let i = 1; i < byLightness.length; i++) {
      if (!used.has(byLightness[i].hex)) {
        result.colors.text = byLightness[i].hex
        used.add(byLightness[i].hex)
        break
      }
    }
  }

  if (hexColors.length >= 4) {
    // accent = most saturated of remaining mid-range colors
    const remaining = hsls.filter((c) => !used.has(c.hex))
    if (remaining.length > 0) {
      remaining.sort((a, b) => b.hsl.s - a.hsl.s)
      result.colors.accent = remaining[0].hex
      used.add(remaining[0].hex)
    }
  }

  // Step 2: Assign semantic roles by hue proximity (priority before badges)
  for (const target of SEMANTIC_TARGETS) {
    const best = findClosest(hsls, target, used)
    if (best) {
      result.colors[target.key] = best
      used.add(best)
    }
  }

  // Step 3: Assign badge colors by hue proximity
  // Badges may reuse colors already assigned to UI/semantic roles
  const badgeUsed = new Set<string>()
  for (const target of BADGE_TARGETS) {
    if (target.isGray) {
      // Gray = lowest saturation (prefer unassigned, but allow reuse)
      const candidates = [...hsls].sort((a, b) => a.hsl.s - b.hsl.s)
      const best = candidates.find((c) => !badgeUsed.has(c.hex))
      if (best) {
        result.badges[target.key] = best.hex
        badgeUsed.add(best.hex)
      }
    } else {
      const best = findClosest(hsls, target, badgeUsed)
      if (best) {
        result.badges[target.key] = best
        badgeUsed.add(best)
      }
    }
  }

  return result
}

/** Find the closest unassigned color to a target HSL. */
function findClosest(
  colors: { hex: string; hsl: HSL }[],
  target: { h: number; s: number; l: number },
  used: Set<string>,
): string | undefined {
  let bestHex: string | undefined
  let bestDist = Infinity

  for (const c of colors) {
    if (used.has(c.hex)) continue
    const d = colorDist(c.hsl, target)
    if (d < bestDist) {
      bestDist = d
      bestHex = c.hex
    }
  }

  return bestHex
}
