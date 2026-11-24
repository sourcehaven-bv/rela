/**
 * Natural-language date input in English and Dutch (TKT-ICADEGW).
 *
 * `parseNaturalDate(text, today)` returns an ISO `YYYY-MM-DD` date, or null
 * when the text is not a date phrase. Both languages are always accepted, so
 * a Dutch reader can type an English phrase and the other way round. Pure:
 * `today` is passed in, so tests pin the day.
 *
 * Accepted phrases (case-insensitive, surrounding whitespace ignored):
 * - ISO dates: 2026-12-01
 * - today, tomorrow, yesterday, day after tomorrow;
 *   vandaag, morgen, gisteren, overmorgen, eergisteren
 * - weekdays: friday, next friday, this friday, last friday;
 *   vrijdag, volgende vrijdag, deze vrijdag, vorige vrijdag (also abbreviations)
 * - offsets: in 3 days, in 2 weeks, in 1 month, 3 days ago, +3d, -1w, +3 (days);
 *   over 3 dagen, over 2 weken, over 1 maand, 3 dagen geleden
 * - next / last week, month, year; volgende (or komende) / vorige week,
 *   maand, jaar (the same day a week, month or year on or back)
 * - day and month: 5 march, march 5, 5 maart, 5 mrt, 5/3, 5-3 (day first),
 *   optionally with a year (four digits after a dash); without one the next
 *   such day from today, which for 29 february may be years away
 * - end of month / einde van de maand
 *
 * Input longer than MAX_PHRASE_LENGTH, and dates outside the years 1 to 9999,
 * give null.
 */

const WEEKDAYS: Record<string, number> = {}
for (const [i, names] of [
  ['sunday', 'sun', 'zondag', 'zo'],
  ['monday', 'mon', 'maandag', 'ma'],
  ['tuesday', 'tue', 'tues', 'dinsdag', 'di'],
  ['wednesday', 'wed', 'woensdag', 'wo'],
  ['thursday', 'thu', 'thur', 'thurs', 'donderdag', 'do'],
  ['friday', 'fri', 'vrijdag', 'vr'],
  ['saturday', 'sat', 'zaterdag', 'za'],
].entries()) {
  for (const n of names) WEEKDAYS[n] = i
}

const MONTHS: Record<string, number> = {}
for (const [i, names] of [
  ['january', 'jan', 'januari'],
  ['february', 'feb', 'februari'],
  ['march', 'mar', 'maart', 'mrt'],
  ['april', 'apr'],
  ['may', 'mei'],
  ['june', 'jun', 'juni'],
  ['july', 'jul', 'juli'],
  ['august', 'aug', 'augustus'],
  ['september', 'sep', 'sept'],
  ['october', 'oct', 'oktober', 'okt'],
  ['november', 'nov'],
  ['december', 'dec'],
].entries()) {
  for (const n of names) MONTHS[n] = i
}

type Unit = 'd' | 'w' | 'm' | 'y'

const UNITS: Record<string, Unit> = {}
for (const [unit, names] of [
  ['d', ['d', 'day', 'days', 'dag', 'dagen']],
  ['w', ['w', 'week', 'weeks', 'weken']],
  ['m', ['m', 'month', 'months', 'maand', 'maanden']],
  ['y', ['y', 'j', 'year', 'years', 'jaar', 'jaren']],
] as [Unit, string[]][]) {
  for (const n of names) UNITS[n] = unit
}

const WORDS: Record<string, number> = {
  today: 0,
  vandaag: 0,
  tomorrow: 1,
  morgen: 1,
  yesterday: -1,
  gisteren: -1,
  overmorgen: 2,
  'day after tomorrow': 2,
  eergisteren: -2,
  'day before yesterday': -2,
}

/** A calendar day, free of time zones. */
interface Day {
  y: number
  m: number // 0-11
  d: number
}

function fromDate(date: Date): Day {
  return { y: date.getFullYear(), m: date.getMonth(), d: date.getDate() }
}

function utc(day: Day): Date {
  return new Date(Date.UTC(day.y, day.m, day.d))
}

/** The ISO date, or null outside the years 1 to 9999 that ISO dates can hold. */
function iso(day: Day): string | null {
  if (day.y < 1 || day.y > 9999) return null
  return utc(day).toISOString().slice(0, 10)
}

function addDays(day: Day, n: number): Day {
  const t = utc(day)
  t.setUTCDate(t.getUTCDate() + n)
  return { y: t.getUTCFullYear(), m: t.getUTCMonth(), d: t.getUTCDate() }
}

function daysIn(y: number, m: number): number {
  return new Date(Date.UTC(y, m + 1, 0)).getUTCDate()
}

/** Adds months, clamping the day to the target month's last day. */
function addMonths(day: Day, n: number): Day {
  const total = day.y * 12 + day.m + n
  const y = Math.floor(total / 12)
  const m = total - y * 12
  return { y, m, d: Math.min(day.d, daysIn(y, m)) }
}

function offset(day: Day, n: number, unit: Unit): Day {
  switch (unit) {
    case 'd':
      return addDays(day, n)
    case 'w':
      return addDays(day, 7 * n)
    case 'm':
      return addMonths(day, n)
    case 'y':
      return addMonths(day, 12 * n)
  }
}

function weekday(day: Day): number {
  return utc(day).getUTCDay()
}

function isoOrNull(day: Day | null): string | null {
  return day && iso(day)
}

/** A valid day, or null for 31 february and the like. */
function valid(y: number, m: number, d: number): Day | null {
  if (m < 0 || m > 11 || d < 1 || d > daysIn(y, m)) return null
  return { y, m, d }
}

/** The day and month, in the given year or else the next one from today. */
function dayMonth(today: Day, d: number, m: number, year?: number): Day | null {
  if (year !== undefined) return valid(year < 100 ? 2000 + year : year, m, d)
  // 29 february can be up to eight years away (2096 to 2104).
  for (let y = today.y; y <= today.y + 8; y++) {
    const day = valid(y, m, d)
    const notPast = y > today.y || m > today.m || (m === today.m && d >= today.d)
    if (day && notPast) return day
  }
  return null
}

/** Longer input is not a date phrase; the cap keeps the work per call small. */
export const MAX_PHRASE_LENGTH = 64

export function parseNaturalDate(text: string, now: Date = new Date()): string | null {
  if (text.length > MAX_PHRASE_LENGTH) return null
  const s = text.trim().toLowerCase().replace(/\s+/g, ' ').replace(/[.,]$/, '')
  if (!s) return null
  const today = fromDate(now)

  let m = /^(\d{4})-(\d{1,2})-(\d{1,2})$/.exec(s)
  if (m) {
    return isoOrNull(valid(+m[1], +m[2] - 1, +m[3]))
  }

  if (s in WORDS) return iso(addDays(today, WORDS[s]))

  // in 3 days, over 2 weken, +3d, -1w; +3 and -3 without a unit are days
  m = /^(?:in|over|\+) ?(\d{1,4}) ?([a-z]+)$/.exec(s) ?? /^\+(\d{1,4})([a-z]*)$/.exec(s)
  if (m && UNITS[m[2] || 'd']) return iso(offset(today, +m[1], UNITS[m[2] || 'd']))
  m = /^(?:-(\d{1,4}) ?([a-z]*)|(\d{1,4}) ([a-z]+) (?:ago|geleden))$/.exec(s)
  if (m) {
    const n = m[1] ?? m[3]
    const unit = UNITS[(m[2] ?? m[4]) || 'd']
    if (unit) return iso(offset(today, -n, unit))
  }

  // next week / volgende maand / last year
  m = /^(next|volgende|komende|last|vorige) (week|month|maand|year|jaar)$/.exec(s)
  if (m) {
    const sign = ['next', 'volgende', 'komende'].includes(m[1]) ? 1 : -1
    return iso(offset(today, sign, UNITS[m[2]]))
  }

  if (/^(end of (the )?month|einde (van de )?maand|eind van de maand)$/.test(s)) {
    return iso({ ...today, d: daysIn(today.y, today.m) })
  }

  // weekdays: friday, next friday, deze vrijdag, vorige vrijdag
  m = /^(?:(next|volgende|this|deze|last|vorige|komende) )?([a-z]+)$/.exec(s)
  if (m && m[2] in WEEKDAYS) {
    const target = WEEKDAYS[m[2]]
    const diff = (target - weekday(today) + 7) % 7
    switch (m[1]) {
      case 'last':
      case 'vorige':
        return iso(addDays(today, diff === 0 ? -7 : diff - 7))
      case 'next':
      case 'volgende':
        // The weekday in the coming week, never today.
        return iso(addDays(today, diff === 0 ? 7 : diff))
      default:
        // friday / this friday / komende vrijdag: today or the next one.
        return iso(addDays(today, diff))
    }
  }

  // 5 march [2027], march 5 [2027]
  m = /^(\d{1,2}) ([a-z]+)(?: (\d{2}|\d{4}))?$/.exec(s)
  if (m && m[2] in MONTHS)
    return isoOrNull(dayMonth(today, +m[1], MONTHS[m[2]], m[3] ? +m[3] : undefined))
  m = /^([a-z]+) (\d{1,2})(?:,? (\d{4}))?$/.exec(s)
  if (m && m[1] in MONTHS)
    return isoOrNull(dayMonth(today, +m[2], MONTHS[m[1]], m[3] ? +m[3] : undefined))

  // 5/3, 5-3, 5/3/2027, 5/3/27, 5-3-2027 (day first). A two-digit year
  // after dashes is refused: 26-11-24 could as well be 2026-11-24.
  m =
    /^(\d{1,2})\/(\d{1,2})(?:\/(\d{2}|\d{4}))?$/.exec(s) ??
    /^(\d{1,2})-(\d{1,2})(?:-(\d{4}))?$/.exec(s)
  if (m) return isoOrNull(dayMonth(today, +m[1], +m[2] - 1, m[3] ? +m[3] : undefined))

  return null
}
