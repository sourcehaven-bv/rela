// useEntitySession: the order of the reads and writes for one entity on
// screen (BUG-04V034W).
//
// Before this, every read of an entity decided on its own whether its answer
// was still wanted, each with its own guard (a view ticket, a load counter, a
// check for pending autosave edits). A read that was sent before one of the
// user's own saves and answered after it fell between those guards, put the
// older body back on screen and made the next save fail as a conflict.
//
// The session owns that decision. A read lands only when
//   (a) the address is the one it was sent for,
//   (b) no read sent after it has landed, and
//   (c) no write to that address completed while it was on its way.
// A read that fails (c) is sent again, a few times at most. Live changes
// (`invalidate`) mark the entity stale and reload it once nothing is written,
// so several of them cost one read.

import { ref, watch, computed, onScopeDispose } from 'vue'
import { shouldDropHeldContent } from '@/api/errors'

// Reads sent again after writes kept completing under them. After that the
// session stays stale and reloads once the writes stop.
export const READ_RETRIES = 3
// A reload that failed transiently is tried again after this, doubling each
// time, at most this many times in a row.
const RETRY_BACKOFF_MS = 1000
const RETRY_BACKOFF_MAX = 5

export type ReadOutcome<T> =
  | { kind: 'landed'; value: T }
  /** The address changed while the read was on its way. */
  | { kind: 'moved' }
  /** A read sent later has already landed, or a refusal came in. */
  | { kind: 'superseded' }
  /** Writes kept completing, or a background read met pending writes; the session reloads later. */
  | { kind: 'stale' }
  /** `prepare` decided against landing. */
  | { kind: 'declined' }
  /** The server refused the read for this address; `onRefused` has run. */
  | { kind: 'refused'; error: unknown }
  /** Any other failure, also one thrown by `prepare` or `land`. `current` is true when no later read has landed. */
  | { kind: 'failed'; error: unknown; current: boolean }

export interface ReadOptions<T> {
  /**
   * Runs once the answer passed the checks, before it lands. It may wait,
   * for example to send edits for the entity being left. The checks run
   * again after it. Returning false keeps the answer off screen.
   */
  prepare?: (value: T) => boolean | Promise<boolean>
  /** Puts the answer on screen. Called at the moment the read lands. */
  land: (value: T) => void
  /**
   * A refresh rather than a load: it does not land while writes are held or
   * on their way (it would replace state the next edit derives from), and
   * the session reloads once they are done. A transient failure is retried
   * after a backoff.
   */
  background?: boolean
  /**
   * Leaves `loading` alone, and a reload it causes is a light one
   * (`reload(false)`). A read that is not quiet causes a full reload.
   */
  quiet?: boolean
}

export interface EntitySessionOptions {
  /**
   * True while the page holds edits the session does not run yet, such as an
   * autosave debounce. Must be reactive: the session reloads when it turns
   * false.
   */
  holdsWrites?: () => boolean
  /**
   * The reload `invalidate` and a stale read run. `full` when any of them
   * asked for one: the page then reloads everything a load does, not only
   * the view.
   */
  reload: (full: boolean) => Promise<unknown>
  /**
   * The server refused a read for the address on screen. Held content must go
   * (frontend/CLAUDE.md, BUG-E8XE7I); a refusal wins over every answer.
   */
  onRefused?: (err: unknown) => void
}

/**
 * One session per entity on screen. `address` names what is on screen (type,
 * id and world); a page that switches entities keeps its session, and answers
 * for the previous address no longer land.
 */
export function useEntitySession(address: () => string, opts: EntitySessionOptions) {
  let issued = 0
  let landed = 0
  // Completed writes per address (rule c). The edits of the entity being
  // left drain after the route has moved on; they must not void the reads of
  // the entity arriving.
  const writesDone = new Map<string, number>()
  const doneFor = (target: string) => writesDone.get(target) ?? 0
  const writing = ref(0)
  let writeTail: Promise<unknown> = Promise.resolve()
  const reading = new Set<{ target: string; ac: AbortController }>()

  // Only the latest non-quiet read clears `loading`.
  const loading = ref(false)
  let loads = 0

  const idle = () => writing.value === 0 && !opts.holdsWrites?.()

  async function read<T>(
    fetch: (signal: AbortSignal) => Promise<T>,
    o: ReadOptions<T>
  ): Promise<ReadOutcome<T>> {
    const target = address()
    const load = o.quiet ? 0 : ++loads
    if (load) loading.value = true
    const full = !o.quiet
    try {
      for (let attempt = 0; ; attempt++) {
        const seq = ++issued
        const writesAtSend = doneFor(target)
        const entry = { target, ac: new AbortController() }
        reading.add(entry)
        let value: T
        try {
          value = await fetch(entry.ac.signal)
        } catch (error) {
          if (address() !== target) return { kind: 'moved' }
          if (shouldDropHeldContent(error)) {
            // Every read sent so far is void, including one waiting in
            // `prepare`: none may put content back once it is dropped.
            landed = issued
            failures = 0
            opts.onRefused?.(error)
            return { kind: 'refused', error }
          }
          const current = seq > landed
          if (o.background && current) retryLater(full)
          return { kind: 'failed', error, current }
        } finally {
          reading.delete(entry)
        }
        // The checks, before and after `prepare`. A write that completed in
        // between voids the answer like one that completed before it came.
        const check = (): ReadOutcome<T> | 'retry' | null => {
          if (address() !== target) return { kind: 'moved' }
          if (seq <= landed) return { kind: 'superseded' }
          if (o.background && !idle()) {
            markStale(full)
            return { kind: 'stale' }
          }
          if (doneFor(target) !== writesAtSend) {
            if (attempt < READ_RETRIES) return 'retry'
            markStale(full)
            return { kind: 'stale' }
          }
          return null
        }
        try {
          let verdict = check()
          if (verdict === 'retry') continue
          if (verdict) return verdict
          if (o.prepare && !(await o.prepare(value))) return { kind: 'declined' }
          verdict = check()
          if (verdict === 'retry') continue
          if (verdict) return verdict
          landed = seq
          failures = 0
          o.land(value)
          return { kind: 'landed', value }
        } catch (error) {
          return { kind: 'failed', error, current: seq > landed }
        }
      }
    } finally {
      if (load === loads) loading.value = false
    }
  }

  /**
   * Runs a write after the writes before it. `target` is the address it
   * writes to, by default the one on screen now; it voids the reads of that
   * address that are on their way.
   *
   * Do not call `write` from inside `run`: the inner write would wait for
   * the outer one. A `run` that never settles blocks every later write, so
   * give it a timeout (autosave's commitImmediately aborts its own).
   */
  function write<T>(run: () => Promise<T>, target = address()): Promise<T> {
    writing.value++
    const result = writeTail.then(run)
    writeTail = result.catch(() => {})
    // Counted when it settles either way: a write that failed on the way back
    // may still have reached the server.
    return result.finally(() => {
      writesDone.set(target, doneFor(target) + 1)
      writing.value--
    })
  }

  // Invalidation generations: `wanted` counts invalidations, `covered` is the
  // generation the last reload was sent for.
  let wanted = 0
  let covered = 0
  let wantFull = false
  let reloading = false

  function invalidate(full = false) {
    markStale(full)
  }

  function markStale(full: boolean) {
    wanted++
    wantFull ||= full
    kick()
  }

  // Transient failures of background reads in a row, and the timer of the
  // next try.
  let failures = 0
  let retryTimer: ReturnType<typeof setTimeout> | undefined
  function retryLater(full: boolean) {
    if (failures >= RETRY_BACKOFF_MAX) return
    const delay = RETRY_BACKOFF_MS * 2 ** failures++
    clearTimeout(retryTimer)
    retryTimer = setTimeout(() => markStale(full), delay)
  }

  function kick() {
    if (reloading || covered === wanted || !idle()) return
    covered = wanted
    const full = wantFull
    wantFull = false
    reloading = true
    // A failed reload is the reload's own business; the session only needs
    // to know it is over.
    void opts
      .reload(full)
      .catch(() => {})
      .finally(() => {
        reloading = false
        kick()
      })
  }

  watch(idle, (now) => {
    if (now) kick()
  })
  // An answer for the address the page has left is not wanted.
  watch(address, (now) => {
    for (const r of reading) if (r.target !== now) r.ac.abort()
  })
  onScopeDispose(() => clearTimeout(retryTimer))

  return {
    read,
    write,
    invalidate,
    loading: computed(() => loading.value),
    /** Writes queued or on their way through `write`. */
    writing: computed(() => writing.value),
  }
}

export type EntitySession = ReturnType<typeof useEntitySession>
