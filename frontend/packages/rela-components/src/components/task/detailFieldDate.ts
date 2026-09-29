/**
 * A detail field stores its date the way it is shown — "18 sep, 2026" — while
 * a date input only speaks `YYYY-MM-DD`. These translate between the two so
 * the field can offer a real picker without changing what it stores.
 *
 * Kept out of the component because parsing is the kind of thing that wants
 * testing directly, and because an app whose dates read differently can
 * substitute its own pair.
 */

const MONTHS = ['jan', 'feb', 'mar', 'apr', 'may', 'jun', 'jul', 'aug', 'sep', 'oct', 'nov', 'dec']

/** `"18 sep, 2026"` to `"2026-09-18"`, or `''` when it does not parse. */
export function toISODate(display: string | undefined): string {
  if (!display) return ''

  // Already ISO: an app that stores dates properly should not be re-parsed.
  const iso = /^(\d{4})-(\d{2})-(\d{2})$/.exec(display.trim())
  if (iso) return display.trim()

  const match = /^(\d{1,2})\s+([a-z]+),?\s+(\d{4})$/i.exec(display.trim())
  if (!match) return ''

  const [, day, monthName, year] = match
  const month = MONTHS.indexOf(monthName.slice(0, 3).toLowerCase())
  if (month === -1) return ''

  return `${year}-${String(month + 1).padStart(2, '0')}-${day.padStart(2, '0')}`
}

/** `"2026-09-18"` back to `"18 sep, 2026"`. */
export function fromISODate(iso: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso)
  if (!match) return ''

  const [, year, month, day] = match
  const name = MONTHS[Number(month) - 1]
  if (!name) return ''

  // No leading zero on the day: "8 sep, 2026" is how the rest of the UI reads.
  return `${Number(day)} ${name}, ${year}`
}
