/**
 * WCAG 2.x contrast math (https://www.w3.org/TR/WCAG21/#dfn-contrast-ratio).
 *
 * Success criteria enforced by tests:
 * - 1.4.3 (AA text): 4.5:1 for normal-size text, 3:1 for large text
 * - 1.4.11 (AA non-text): 3:1 for meaningful graphics against adjacent colors
 */

/** srgbChannel linearizes one 0-255 channel per the WCAG formula. */
function srgbChannel(v: number): number {
  const c = v / 255
  return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)
}

/** relativeLuminance of a "#rrggbb" color. */
export function relativeLuminance(hex: string): number {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex)
  if (!m) throw new Error(`not a #rrggbb color: ${hex}`)
  const n = parseInt(m[1], 16)
  return (
    0.2126 * srgbChannel((n >> 16) & 0xff) +
    0.7152 * srgbChannel((n >> 8) & 0xff) +
    0.0722 * srgbChannel(n & 0xff)
  )
}

/** contrastRatio between two "#rrggbb" colors, 1..21. */
export function contrastRatio(a: string, b: string): number {
  const la = relativeLuminance(a)
  const lb = relativeLuminance(b)
  const [hi, lo] = la >= lb ? [la, lb] : [lb, la]
  return (hi + 0.05) / (lo + 0.05)
}

/** flatten composites an rgba color over an opaque background, so an
 * alpha-blended fill can be contrast-checked as what the eye actually sees. */
export function flatten(fg: string, alpha: number, bg: string): string {
  const p = (hex: string) => {
    const n = parseInt(/^#?([0-9a-f]{6})$/i.exec(hex)![1], 16)
    return [(n >> 16) & 0xff, (n >> 8) & 0xff, n & 0xff]
  }
  const f = p(fg)
  const b = p(bg)
  const mix = f.map((c, i) => Math.round(c * alpha + b[i] * (1 - alpha)))
  return '#' + mix.map((c) => c.toString(16).padStart(2, '0')).join('')
}
