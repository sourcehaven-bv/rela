/**
 * Exact decimal values (TKT-DTUAG3U).
 *
 * A `decimal` property is stored as text in plain decimal notation, such as
 * "12.50". A JavaScript number cannot hold 0.1 exactly, so arithmetic on
 * decimals here works on a scaled BigInt: "12.50" is 1250 with scale 2.
 * The server reads the same notation (metamodel.PropertyTypeDecimal).
 */

/** A decimal as `units / 10^scale`. */
export interface Decimal {
  units: bigint
  scale: number
}

const DECIMAL_TEXT_RE = /^([+-]?)(\d+)(?:\.(\d+))?$/

// The server refuses longer values; the cap also bounds BigInt work here.
const MAX_DECIMAL_TEXT = 64

/**
 * A stored decimal value, or undefined when it is not one. Decimal text is
 * the normal form; a JSON number is accepted too, read through its shortest
 * text, as the server reads an unquoted YAML number.
 */
export function parseDecimal(value: unknown): Decimal | undefined {
  let text: string
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) return undefined
    text = numberText(value)
  } else if (typeof value === 'string') {
    text = value
  } else {
    return undefined
  }
  if (text.length > MAX_DECIMAL_TEXT) return undefined
  const m = DECIMAL_TEXT_RE.exec(text)
  if (!m) return undefined
  const frac = m[3] ?? ''
  const units = BigInt(m[2] + frac)
  return { units: m[1] === '-' ? -units : units, scale: frac.length }
}

// A number's plain decimal text. String(n) uses an exponent for very large
// and very small numbers, which decimal notation does not allow.
function numberText(n: number): string {
  const s = String(n)
  if (!/e/i.test(s)) return s
  return n.toLocaleString('en-US', { useGrouping: false, maximumFractionDigits: 20 })
}

/** The decimal as text with exactly `scale` digits after the point. */
export function decimalText(d: Decimal): string {
  const negative = d.units < 0n
  const digits = (negative ? -d.units : d.units).toString().padStart(d.scale + 1, '0')
  const sign = negative ? '-' : ''
  if (d.scale === 0) return sign + digits
  return `${sign}${digits.slice(0, -d.scale)}.${digits.slice(-d.scale)}`
}

function rescale(d: Decimal, scale: number): bigint {
  return d.units * 10n ** BigInt(scale - d.scale)
}

/** The exact sum of the decimals, at the largest scale among them. */
export function sumDecimals(values: readonly Decimal[]): Decimal {
  const scale = Math.max(0, ...values.map((v) => v.scale))
  let units = 0n
  for (const v of values) units += rescale(v, scale)
  return { units, scale }
}

/** a < b: negative; equal: 0; a > b: positive. */
export function compareDecimals(a: Decimal, b: Decimal): number {
  const scale = Math.max(a.scale, b.scale)
  const x = rescale(a, scale)
  const y = rescale(b, scale)
  return x < y ? -1 : x > y ? 1 : 0
}

/**
 * d divided by a positive whole number, rounded half away from zero to
 * `scale` digits after the point.
 */
export function divideDecimal(d: Decimal, divisor: number, scale: number): Decimal {
  // d / divisor at `scale` is (units * 10^scale) / (divisor * 10^d.scale).
  const num = d.units * 10n ** BigInt(scale)
  const den = BigInt(divisor) * 10n ** BigInt(d.scale)
  let q = num / den
  const r = num % den
  // A remainder of at least half the divisor rounds away from zero.
  if ((r < 0n ? -r : r) * 2n >= den) q += r < 0n ? -1n : 1n
  return { units: q, scale }
}
