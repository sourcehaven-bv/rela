// Language-neutral parser for typed dates. It knows a fixed set of phrase
// shapes and takes every word from a DatePack (types.ts):
//
//   2026-03-05            ISO date
//   tomorrow              a relativeDays phrase
//   friday, this friday   on or after today
//   next friday           strictly after today
//   last friday           strictly before today
//   in 3 days, 3 days     ahead by N units
//   3 days ago            back by N units
//
// Modifier and direction words may lead or trail, so word order is a property
// of the pack, not of this file.
//   next week, last month one unit ahead or back
//   5 march [2027]        day and month; the next occurrence without a year
//   march 5th [2027]      the same, month first
//
// Input is matched token by token, never by a backtracking regex over the
// whole string, and is capped in length, so cost is linear and small.
// Arithmetic runs on UTC calendar days, so a DST change cannot shift a date.
import type { DatePack } from './types'
import { DATE_PACKS, FALLBACK_PACK } from './locales'

const MAX_INPUT = 64
const MAX_COUNT = 999

type Unit = 'day' | 'week' | 'month' | 'year'

/** Packs to try for a UI language: its own pack, then the fallback. */
export function packsFor(language: string | undefined): DatePack[] {
  const base = (language ?? '').toLowerCase().split('-')[0]
  const own = DATE_PACKS[base]
  return own && own !== FALLBACK_PACK ? [own, FALLBACK_PACK] : [FALLBACK_PACK]
}

/**
 * Resolves `input` to a `YYYY-MM-DD` string relative to `today` (local date),
 * trying each pack in order. Returns null when no shape matches or the date
 * does not exist.
 */
export function parseNaturalDate(input: string, today: Date, packs: DatePack[]): string | null {
  if (input.length > MAX_INPUT) return null
  const folded = fold(input)
  if (folded === '') return null
  const base = Date.UTC(today.getFullYear(), today.getMonth(), today.getDate())
  const iso = /^(\d{4})-(\d{2})-(\d{2})$/.exec(folded)
  if (iso) {
    const ms = makeDate(Number(iso[1]), Number(iso[2]) - 1, Number(iso[3]))
    return ms === null ? null : format(ms)
  }
  for (const pack of packs) {
    const ms = parseWith(pack, folded, base)
    if (ms !== null) return format(ms)
  }
  return null
}

/** Lowercases, strips diacritics and punctuation, and collapses whitespace. */
export function fold(input: string): string {
  return input
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[,.]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function splitSpaces(folded: string): string[] {
  return folded.split(' ').filter((t) => t !== '')
}

/** Every word of a pack that is written with spaces, longest first. */
function multiWordPhrases(pack: DatePack): string[][] {
  const words = [
    ...pack.this,
    ...pack.next,
    ...pack.last,
    ...pack.in,
    ...pack.ago,
    ...pack.weekdays.flat(),
    ...pack.months.flat(),
    ...Object.values(pack.units).flat(),
  ]
  return words
    .filter((w) => w.includes(' '))
    .map((w) => w.split(' '))
    .sort((a, b) => b.length - a.length)
}

/**
 * Tokenizes for one pack: splits, joins the pack's multi-word words back into
 * single tokens, and drops filler words.
 */
function tokensFor(pack: DatePack, folded: string): string[] {
  const raw = (pack.tokenize ?? splitSpaces)(folded)
  const phrases = multiWordPhrases(pack)
  const out: string[] = []
  for (let i = 0; i < raw.length;) {
    const hit = phrases.find((parts) => parts.every((part, k) => raw[i + k] === part))
    if (hit) {
      out.push(hit.join(' '))
      i += hit.length
    } else {
      out.push(raw[i])
      i += 1
    }
  }
  const fillers = pack.fillers ?? []
  return out.filter((t) => !fillers.includes(t))
}

function parseWith(pack: DatePack, folded: string, base: number): number | null {
  const days = new Map(Object.entries(pack.relativeDays))
  const whole = days.get(folded)
  if (whole !== undefined) return addDays(base, whole)

  const tokens = tokensFor(pack, folded)
  if (tokens.length === 0) return null
  const joined = days.get(tokens.join(' '))
  if (joined !== undefined) return addDays(base, joined)

  // A modifier may lead or trail: "next friday", "vendredi prochain".
  let mod = modifier(pack, tokens[0])
  let rest = tokens.slice(1)
  if (!mod && tokens.length > 1) {
    mod = modifier(pack, tokens[tokens.length - 1])
    rest = tokens.slice(0, -1)
  }
  if (!mod) rest = tokens

  if (rest.length === 1) {
    const wd = indexIn(pack.weekdays, rest[0])
    if (wd >= 0) return weekday(base, wd, mod ?? 'this')
    const unit = unitOf(pack, rest[0])
    if (unit && mod && mod !== 'this') return addUnits(base, unit, mod === 'next' ? 1 : -1)
  }

  const counted = countPhrase(pack, tokens)
  if (counted) return addUnits(base, counted.unit, counted.sign * counted.n)

  // Day and month, optionally beside a `this` word such as "on".
  return dayMonth(pack, mod === 'this' ? rest : tokens, base)
}

function modifier(pack: DatePack, token: string): 'this' | 'next' | 'last' | null {
  if (pack.this.includes(token)) return 'this'
  if (pack.next.includes(token)) return 'next'
  if (pack.last.includes(token)) return 'last'
  return null
}

function indexIn(lists: string[][], token: string): number {
  return lists.findIndex((names) => names.includes(token))
}

function unitOf(pack: DatePack, token: string): Unit | null {
  for (const unit of ['day', 'week', 'month', 'year'] as const) {
    if (pack.units[unit].includes(token)) return unit
  }
  return null
}

function count(pack: DatePack, token: string): number | null {
  const n = /^\d{1,3}$/.test(token) ? Number(token) : pack.numbers?.[token]
  return n !== undefined && n >= 1 && n <= MAX_COUNT ? n : null
}

function countPhrase(
  pack: DatePack,
  tokens: string[]
): { n: number; unit: Unit; sign: 1 | -1 } | null {
  let t = tokens
  let sign: 1 | -1 = 1
  if (t.length === 3) {
    // The direction word may lead or trail: "in 3 days", "3 days ago", "vor 3 tagen".
    const [head, , tail] = t
    if (pack.in.includes(head) || pack.ago.includes(head)) {
      sign = pack.ago.includes(head) ? -1 : 1
      t = t.slice(1)
    } else if (pack.in.includes(tail) || pack.ago.includes(tail)) {
      sign = pack.ago.includes(tail) ? -1 : 1
      t = t.slice(0, 2)
    } else return null
  }
  if (t.length !== 2) return null
  const n = count(pack, t[0])
  const unit = unitOf(pack, t[1])
  return n !== null && unit ? { n, unit, sign } : null
}

function dayNumber(pack: DatePack, token: string): number | null {
  if (/^\d{1,2}$/.test(token)) return Number(token)
  // Longest suffix first, so Dutch "1ste" is not read as "1st" plus "e".
  const suffixes = [...(pack.ordinals ?? [])].sort((a, b) => b.length - a.length)
  for (const suffix of suffixes) {
    const digits = token.slice(0, -suffix.length)
    if (token.endsWith(suffix) && /^\d{1,2}$/.test(digits)) return Number(digits)
  }
  return null
}

function dayMonth(pack: DatePack, tokens: string[], base: number): number | null {
  if (tokens.length < 2 || tokens.length > 3) return null
  let day = dayNumber(pack, tokens[0])
  let month = indexIn(pack.months, tokens[1])
  if (day === null || month < 0) {
    day = dayNumber(pack, tokens[1])
    month = indexIn(pack.months, tokens[0])
  }
  if (day === null || month < 0) return null
  if (tokens.length === 3) {
    if (!/^\d{4}$/.test(tokens[2])) return null
    return makeDate(Number(tokens[2]), month, day)
  }
  const year = new Date(base).getUTCFullYear()
  const thisYear = makeDate(year, month, day)
  if (thisYear !== null && thisYear >= base) return thisYear
  // Past this year, or 29 February in a non-leap year: the next occurrence.
  for (let y = year + 1; y <= year + 8; y++) {
    const ms = makeDate(y, month, day)
    if (ms !== null) return ms
  }
  return null
}

const DAY_MS = 86_400_000

function addDays(base: number, days: number): number {
  return base + days * DAY_MS
}

function weekday(base: number, target: number, mode: 'this' | 'next' | 'last'): number {
  const current = (new Date(base).getUTCDay() + 6) % 7 // Monday is 0
  if (mode === 'last') return addDays(base, -(((current - target + 6) % 7) + 1))
  const ahead = (target - current + 7) % 7
  return addDays(base, mode === 'next' && ahead === 0 ? 7 : ahead)
}

function addUnits(base: number, unit: Unit, n: number): number | null {
  if (unit === 'day') return addDays(base, n)
  if (unit === 'week') return addDays(base, 7 * n)
  const d = new Date(base)
  const months = unit === 'year' ? 12 * n : n
  const total = d.getUTCFullYear() * 12 + d.getUTCMonth() + months
  const year = Math.floor(total / 12)
  const month = total - year * 12
  // Clamp to the month's last day: 31 January plus one month is 28 or 29 February.
  const last = new Date(Date.UTC(year, month + 1, 0)).getUTCDate()
  return makeDate(year, month, Math.min(d.getUTCDate(), last))
}

/** Returns the UTC midnight of a real calendar date, or null. */
function makeDate(year: number, month: number, day: number): number | null {
  if (year < 1 || year > 9999 || day < 1) return null
  const d = new Date(Date.UTC(year, month, day))
  d.setUTCFullYear(year) // Date.UTC maps years below 100 to 19xx
  return d.getUTCMonth() === month && d.getUTCDate() === day ? d.getTime() : null
}

function format(ms: number): string {
  return new Date(ms).toISOString().slice(0, 10)
}
