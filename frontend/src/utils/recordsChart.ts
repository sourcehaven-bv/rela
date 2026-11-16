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
 */
export function rangeStart(range: string | undefined, now: Date): string | undefined {
  const m = /^(\d+)([hdwy])$/.exec((range ?? '').trim())
  if (!m) return undefined
  const n = Number(m[1])
  if (n <= 0) return undefined
  return new Date(now.getTime() - n * DURATION_UNITS[m[2]]).toISOString()
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
    if (pts?.length) out.push({ owner, slot, points: pts.sort((a, b) => a.t - b.t) })
  })
  return out
}

/** The distinct bucket starts across all series, ascending. */
export function bucketStarts(series: Series[]): number[] {
  const set = new Set<number>()
  for (const s of series) for (const p of s.points) set.add(p.t)
  return [...set].sort((a, b) => a - b)
}

/**
 * Round y-axis ticks covering [min, max]. The axis includes zero when every
 * value is on one side of it, so a line does not exaggerate a small change.
 */
export function niceTicks(min: number, max: number, count = 4): number[] {
  let lo = Math.min(0, min)
  let hi = Math.max(0, max)
  if (lo === hi) hi = lo + 1
  const raw = (hi - lo) / count
  const mag = 10 ** Math.floor(Math.log10(raw))
  const step = [1, 2, 2.5, 5, 10].map((f) => f * mag).find((s) => s >= raw) ?? 10 * mag
  lo = Math.floor(lo / step) * step
  hi = Math.ceil(hi / step) * step
  const ticks: number[] = []
  for (let v = lo; v <= hi + step / 2; v += step) ticks.push(Number(v.toPrecision(12)))
  return ticks
}

/** Formats a bucket start for its bucket, in UTC: buckets are UTC periods. */
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
