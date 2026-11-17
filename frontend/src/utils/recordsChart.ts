/**
 * Pure helpers for the `display: line` dashboard card (TKT-RK810V7): turn the
 * points of `/_records/{type}/_aggregate?per_owner=true` into series and
 * scales. No DOM and no fetching, so the folding rules are unit-tested here.
 */
import type { DashboardRecords } from '@/types'
import type { RecordPoint } from '@/api/records'

/** The bucket a card uses when it names none. */
export const DEFAULT_BUCKET = 'month'

/** The statistic a card uses when it names none. */
export const DEFAULT_STAT = 'latest'

/** One plotted bucket of one owner. */
export interface SeriesPoint {
  /** Bucket start, epoch milliseconds. */
  t: number
  v: number
}

/** One line: the buckets of one owner the reader may read. */
export interface Series {
  owner: string
  /**
   * The owner's position in the card's `owners:`, which picks its colour.
   * Colour follows the owner, never its rank among the visible lines, so a
   * line the reader cannot see does not repaint the others.
   */
  slot: number
  points: SeriesPoint[]
}

const HOUR = 3_600_000
const DURATION_UNITS: Record<string, number> = {
  h: HOUR,
  d: 24 * HOUR,
  w: 7 * 24 * HOUR,
  y: 365 * 24 * HOUR,
}

/**
 * The start of a card's `range:` before `now`, as RFC 3339, or undefined for
 * no range. The units are those of a record retention (48h, 90d, 2w, 5y),
 * which the server validated at config load; anything else is no range.
 *
 * The start is moved back to the start of its bucket, so the first bucket
 * holds a whole period. Otherwise a sum or a count would dip in the first
 * bucket for no reason in the data.
 */
export function rangeStart(
  range: string | undefined,
  now: Date,
  bucket: DashboardRecords['bucket'] = DEFAULT_BUCKET
): string | undefined {
  const m = /^(\d+)([hdwy])$/.exec((range ?? '').trim())
  if (!m) return undefined
  const n = Number(m[1])
  if (n <= 0) return undefined
  return new Date(bucketStart(now.getTime() - n * DURATION_UNITS[m[2]], bucket)).toISOString()
}

/**
 * The start of the bucket holding t, in UTC, as the server cuts buckets:
 * whole hours, days, ISO weeks from Monday, and calendar months.
 */
export function bucketStart(t: number, bucket: DashboardRecords['bucket']): number {
  const d = new Date(t)
  switch (bucket ?? DEFAULT_BUCKET) {
    case 'hour':
      return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate(), d.getUTCHours())
    case 'day':
      return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate())
    case 'week': {
      const back = (d.getUTCDay() + 6) % 7
      return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate() - back)
    }
    default:
      return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), 1)
  }
}

/** The start of the bucket after the one starting at t. */
export function nextBucket(t: number, bucket: DashboardRecords['bucket']): number {
  const d = new Date(t)
  switch (bucket ?? DEFAULT_BUCKET) {
    case 'hour':
      return t + HOUR
    case 'day':
      return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate() + 1)
    case 'week':
      return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate() + 7)
    default:
      return Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + 1, 1)
  }
}

/**
 * Splits a series' points into runs of consecutive buckets. A line is drawn
 * through each run only: a bucket without rows has no value, and a straight
 * line across it would show one.
 */
export function segments(
  points: SeriesPoint[],
  bucket: DashboardRecords['bucket']
): SeriesPoint[][] {
  const out: SeriesPoint[][] = []
  for (const p of points) {
    const run = out[out.length - 1]
    const prev = run?.[run.length - 1]
    if (prev && nextBucket(prev.t, bucket) === p.t) run.push(p)
    else out.push([p])
  }
  return out
}

/** The value of point p for the card's statistic, or undefined when absent. */
export function pointValue(p: RecordPoint, stat: DashboardRecords['stat']): number | undefined {
  const s = stat ?? DEFAULT_STAT
  if (s === 'count') return p.count
  const raw = p[s]
  if (raw === undefined || raw === '') return undefined
  const v = Number(raw)
  return Number.isFinite(v) ? v : undefined
}

/**
 * Groups points into one series per owner, in the card's owner order.
 *
 * An owner the server did not return (because the reader may not read it, or
 * it has no rows in range) has no series: nothing is drawn or named for it.
 * Points without a start, or for an owner the card does not name, are
 * dropped.
 */
export function buildSeries(cfg: DashboardRecords, points: RecordPoint[]): Series[] {
  const byOwner = new Map<string, SeriesPoint[]>()
  for (const p of points) {
    if (!p.owner || !p.start || !cfg.owners.includes(p.owner)) continue
    const v = pointValue(p, cfg.stat)
    const t = Date.parse(p.start)
    if (v === undefined || Number.isNaN(t)) continue
    let list = byOwner.get(p.owner)
    if (!list) byOwner.set(p.owner, (list = []))
    list.push({ t, v })
  }
  const out: Series[] = []
  cfg.owners.forEach((owner, slot) => {
    const pts = byOwner.get(owner)
    // An owner named twice keeps its first slot; the server refuses such a
    // card at load, so this only guards a hand-crafted response.
    if (pts?.length && !out.some((s) => s.owner === owner)) {
      out.push({ owner, slot, points: pts.sort((a, b) => a.t - b.t) })
    }
  })
  return out
}

/** The distinct bucket starts across all series, ascending. */
export function bucketStarts(series: Series[]): number[] {
  const set = new Set<number>()
  for (const s of series) for (const p of s.points) set.add(p.t)
  return [...set].sort((a, b) => a - b)
}

/** The decimal places needed to write x exactly, up to 10. */
export function decimalsOf(x: number): number {
  let d = 0
  while (d < 10 && Math.abs(Math.round(x * 10 ** d) - x * 10 ** d) > 1e-6) d++
  return d
}

/**
 * Round y-axis ticks covering [min, max]. The axis includes zero when every
 * value is on one side of it, so a line does not exaggerate a small change.
 * Each tick is computed from its index and rounded to the step's decimals,
 * so zero is exactly 0 and not a float residue.
 */
export function niceTicks(min: number, max: number, count = 4): number[] {
  let lo = Math.min(0, min)
  let hi = Math.max(0, max)
  if (lo === hi) hi = lo + 1
  const raw = (hi - lo) / count
  const mag = 10 ** Math.floor(Math.log10(raw))
  const step = [1, 2, 2.5, 5, 10].map((f) => f * mag).find((s) => s >= raw) ?? 10 * mag
  const places = decimalsOf(step)
  lo = Math.floor(lo / step) * step
  hi = Math.ceil(hi / step) * step
  const n = Math.round((hi - lo) / step)
  return Array.from({ length: n + 1 }, (_, i) => Number((lo + i * step).toFixed(places)) || 0)
}

/**
 * A formatter for the y-axis ticks that writes every tick exactly. Large
 * values use compact notation (12k) with as many decimals as the step needs
 * in that unit, so 12500 reads 12.5k and not 13k.
 */
export function tickFormatter(ticks: number[], locale: string): (v: number) => string {
  const step = ticks.length > 1 ? ticks[1] - ticks[0] : 1
  const maxAbs = Math.max(0, ...ticks.map(Math.abs))
  if (maxAbs < 10_000) {
    const f = new Intl.NumberFormat(locale, { maximumFractionDigits: decimalsOf(step) })
    return (v) => f.format(v)
  }
  const unit = 10 ** (3 * Math.floor(Math.log10(maxAbs) / 3))
  const f = new Intl.NumberFormat(locale, {
    notation: 'compact',
    maximumFractionDigits: decimalsOf(step / unit),
  })
  return (v) => f.format(v)
}

/**
 * Formats a bucket start for its bucket, in UTC: buckets are UTC periods, so
 * an hour bucket reads as a UTC hour.
 */
export function formatBucket(
  t: number,
  bucket: DashboardRecords['bucket'],
  locale: string
): string {
  const opts: Intl.DateTimeFormatOptions = { timeZone: 'UTC' }
  switch (bucket ?? DEFAULT_BUCKET) {
    case 'hour':
      Object.assign(opts, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
      break
    case 'day':
    case 'week':
      Object.assign(opts, { day: 'numeric', month: 'short', year: 'numeric' })
      break
    default:
      Object.assign(opts, { month: 'short', year: 'numeric' })
  }
  return new Intl.DateTimeFormat(locale, opts).format(new Date(t))
}
