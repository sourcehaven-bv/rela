/**
 * The `@` completion menu's state machine, shared by both editors.
 *
 * The SPA form and the sandboxed app editor run the same menu: same trigger
 * length, same debounce, same result cap, same staleness rule. They differ only
 * in how they FETCH (axios with an `AbortController` vs the app bridge, which
 * offers no cancellation) and in how they RENDER (Vue vs plain DOM). Both of
 * those are injected, so neither can pull a framework in here.
 *
 * It lives in one place because the first version did not. The app editor
 * started as a hand-written copy of this logic, and a copy of a state machine
 * is worse than a copy of a stylesheet: the two menus could quietly disagree
 * about when to search and how a slow response is resolved, and nothing would
 * say so. That is the same defect (RR-9PTXV0) that TKT-D2JML7 was written to
 * remove, so shipping a smaller instance of it was not an option.
 */
import { rankByIdMatch } from './rankMentions'

/**
 * Below this length the menu prompts rather than searching.
 *
 * The backtick flow this replaced could list a type's entities on an empty
 * query, because the user had already chosen a type. `@` has no type to scope
 * by, and there is no cross-type listing endpoint: `listEntities` resolves a
 * per-type URL and throws on an unknown type. Searching for one character
 * across every type returns relevance-scored noise.
 */
export const MIN_SEARCH_LEN = 2
const SEARCH_DEBOUNCE_MS = 150
const MAX_RESULTS = 20

/** The menu's observable state. Plain data, so any renderer can read it. */
export interface MentionMenuSnapshot<T> {
  /** True while the menu should be on screen. */
  open: boolean
  /** The query text after the `@`. */
  query: string
  items: T[]
  highlightedIndex: number
  loading: boolean
  errorMsg: string
}

/**
 * Fetches candidates for a query.
 *
 * `signal` is passed when the caller's transport can cancel. The app bridge
 * cannot, so it ignores the argument and relies on the generation guard below,
 * which is why cancellation is an optimization here and never the correctness
 * mechanism.
 *
 * Nil: a rejection other than an abort is reported to the user as a failed
 * search; an abort is swallowed.
 */
export type MentionSearchFn<T> = (query: string, signal?: AbortSignal) => Promise<T[]>

export interface MentionMenuMachine<T> {
  readonly state: Readonly<MentionMenuSnapshot<T>>
  /** The shortest query that triggers a search; below it the menu prompts. */
  readonly minQueryLength: number
  /** Opens the menu, or updates the query if it is already open. */
  setQuery(query: string): void
  close(): void
  moveHighlight(delta: number): void
  setHighlight(index: number): void
  /** The item under the highlight, or null when there is nothing to pick. */
  current(): T | null
  dispose(): void
}

export interface MentionMenuOptions<T> {
  search: MentionSearchFn<T>
  /** Called after every state change, so the renderer can repaint. */
  onChange?: () => void
  /**
   * Whether the transport supports cancellation.
   *
   * When true an `AbortController` is created and its signal passed to
   * `search`, and an `AbortError` rejection is ignored rather than reported.
   */
  abortable?: boolean
}

/**
 * Builds the menu's state machine.
 *
 * The returned `state` object is MUTATED in place rather than replaced, so a
 * Vue caller can wrap it in `reactive()` and a plain-DOM caller can read it
 * directly after `onChange`.
 */
export function createMentionMenuMachine<T extends { id?: string }>(
  options: MentionMenuOptions<T>
): MentionMenuMachine<T> {
  const state: MentionMenuSnapshot<T> = {
    open: false,
    query: '',
    items: [],
    highlightedIndex: 0,
    loading: false,
    errorMsg: '',
  }

  let searchTimer: ReturnType<typeof setTimeout> | null = null
  let abort: AbortController | null = null
  let disposed = false
  // Guards against a slow response for an old query landing after a newer one.
  // This is the load-bearing staleness defence: abort is an optimization the
  // bridge transport does not have.
  let generation = 0

  const changed = (): void => options.onChange?.()

  function cancelPending(): void {
    if (searchTimer) {
      clearTimeout(searchTimer)
      searchTimer = null
    }
    abort?.abort()
    abort = null
  }

  async function runSearch(query: string, gen: number): Promise<void> {
    let signal: AbortSignal | undefined
    if (options.abortable) {
      abort = new AbortController()
      signal = abort.signal
    }
    state.loading = true
    changed()
    try {
      const items = await options.search(query, signal)
      if (disposed || gen !== generation) return
      state.items = rankByIdMatch(items, query).slice(0, MAX_RESULTS)
      state.errorMsg = ''
      state.highlightedIndex = 0
    } catch (err: unknown) {
      if ((err as { name?: string })?.name === 'AbortError') return
      if (disposed || gen !== generation) return
      state.items = []
      state.errorMsg = 'Search failed'
    } finally {
      if (!disposed && gen === generation) {
        state.loading = false
        changed()
      }
    }
  }

  return {
    state,
    minQueryLength: MIN_SEARCH_LEN,

    setQuery(query: string): void {
      if (disposed) return
      state.open = true
      state.query = query
      cancelPending()
      const gen = ++generation
      if (query.length < MIN_SEARCH_LEN) {
        // Show the prompt rather than a stale result set from a longer query
        // the user has just backspaced away from.
        state.items = []
        state.loading = false
        state.errorMsg = ''
        state.highlightedIndex = 0
        changed()
        return
      }
      changed()
      searchTimer = setTimeout(() => {
        searchTimer = null
        void runSearch(query, gen)
      }, SEARCH_DEBOUNCE_MS)
    },

    close(): void {
      cancelPending()
      // Bump the generation so an in-flight response cannot reopen the menu.
      generation++
      state.open = false
      state.query = ''
      state.items = []
      state.highlightedIndex = 0
      state.loading = false
      state.errorMsg = ''
      changed()
    },

    moveHighlight(delta: number): void {
      if (state.items.length === 0) return
      const n = state.items.length
      state.highlightedIndex = (state.highlightedIndex + delta + n) % n
      changed()
    },

    setHighlight(index: number): void {
      if (index < 0 || index >= state.items.length) return
      state.highlightedIndex = index
      changed()
    },

    current(): T | null {
      return state.items[state.highlightedIndex] ?? null
    },

    dispose(): void {
      disposed = true
      cancelPending()
    },
  }
}
