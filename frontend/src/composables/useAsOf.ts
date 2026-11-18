/**
 * The instant the page reads at, synced to the `?as_of=` URL query param
 * (TKT-6KLUT3U). Absent means the present.
 *
 * Like the world, the instant is in the URL so "the statement of
 * applicability on 31 December" is a link an auditor can be sent. Unlike the
 * world, changing it reloads the page: every cached read on the page was made
 * for the other instant, and a reload is the one way to drop all of them at
 * once. The API client applies the instant to every read (`api/asOf.ts`), and
 * the router carries it to the next page (`router/index.ts`).
 */
import { computed, type ComputedRef } from 'vue'
import { currentAsOf } from '@/api/asOf'

export interface UseAsOf {
  /** The instant the page reads at, or undefined for the present. */
  asOf: ComputedRef<string | undefined>
  /** Reads at `instant` (RFC 3339 with a zone), or the present when undefined. */
  setAsOf: (instant: string | undefined) => void
}

export function useAsOf(): UseAsOf {
  const asOf = computed(() => currentAsOf())

  function setAsOf(instant: string | undefined) {
    if (instant === currentAsOf()) return
    const url = new URL(window.location.href)
    if (instant) url.searchParams.set('as_of', instant)
    else url.searchParams.delete('as_of')
    url.searchParams.delete('page')
    window.location.assign(url.toString())
  }

  return { asOf, setAsOf }
}

/**
 * The instant at the end of `date` (YYYY-MM-DD) in the browser's time zone,
 * as RFC 3339 with that zone's offset. "The state on 31 December" is the
 * state at the close of that day.
 */
export function endOfDay(date: string): string | undefined {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date)
  if (!m) return undefined
  const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 23, 59, 59, 999)
  if (Number.isNaN(d.getTime())) return undefined
  const offset = -d.getTimezoneOffset()
  const sign = offset >= 0 ? '+' : '-'
  const abs = Math.abs(offset)
  const pad = (n: number, w = 2) => String(n).padStart(w, '0')
  return (
    `${m[1]}-${m[2]}-${m[3]}T23:59:59.999` + `${sign}${pad(Math.floor(abs / 60))}:${pad(abs % 60)}`
  )
}

/** The local calendar date (YYYY-MM-DD) of an instant, for the date input. */
export function localDate(instant: string): string {
  const d = new Date(instant)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
