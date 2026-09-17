/**
 * The `@` completion menu's state and search.
 *
 * The old backtick flow made the user pick a type prefix first, then an
 * entity, because a backtick carries no information about what is wanted. The
 * `@` trigger drops that step: it searches every type at once, which is what
 * the user asked for when they said they wanted easy inline link insertion.
 *
 * This composable owns only state and search. Where the trigger sits and how
 * the result gets inserted belong to the editor, so nothing here touches
 * ProseMirror.
 */
import { reactive, readonly, type DeepReadonly } from 'vue'
import { searchEntities } from '@/api'
import type { Entity } from '@/types'

/**
 * Below this length the menu shows nothing rather than searching.
 *
 * The backtick flow could list a type's entities on an empty query because the
 * user had already chosen a type. `@` has no type to scope by, and there is no
 * cross-type listing endpoint: `listEntities` resolves a per-type URL and
 * throws on an unknown type. Searching for one character across every type
 * returns relevance-scored noise, so the menu prompts instead.
 */
const MIN_SEARCH_LEN = 2
const SEARCH_DEBOUNCE_MS = 150
const MAX_RESULTS = 20

export interface MentionMenuState {
  /** True while the menu should be on screen. */
  open: boolean
  /** The query text after the `@`. */
  query: string
  items: Entity[]
  highlightedIndex: number
  loading: boolean
  errorMsg: string
}

export interface MentionMenuController {
  state: DeepReadonly<MentionMenuState>
  /** The shortest query that triggers a search; below it the menu prompts. */
  minQueryLength: number
  /** Opens the menu, or updates the query if it is already open. */
  setQuery: (query: string) => void
  close: () => void
  moveHighlight: (delta: number) => void
  setHighlight: (index: number) => void
  /** The entity under the highlight, or null when there is nothing to pick. */
  current: () => Entity | null
  dispose: () => void
}

/**
 * Ranks exact and prefix ID matches ahead of loose title matches.
 *
 * The search backend scores by title-token relevance, so typing an ID prefix
 * buries the entity the user is plainly aiming at. Ranking is stable within a
 * tier, so the backend's relevance order survives as the tie-break.
 */
export function rankByIdMatch(items: Entity[], query: string): Entity[] {
  if (!query) return items
  const q = query.toUpperCase()
  const tier = (e: Entity): number => {
    const id = (e.id ?? '').toUpperCase()
    if (id === q) return 0
    if (id.startsWith(q)) return 1
    if (id.includes(q)) return 2
    return 3
  }
  return items
    .map((e, i) => ({ e, i, t: tier(e) }))
    .sort((a, b) => a.t - b.t || a.i - b.i)
    .map(({ e }) => e)
}

export function useMentionMenu(): MentionMenuController {
  const state = reactive<MentionMenuState>({
    open: false,
    query: '',
    items: [],
    highlightedIndex: 0,
    loading: false,
    errorMsg: '',
  })

  let searchTimer: ReturnType<typeof setTimeout> | null = null
  let abort: AbortController | null = null
  let disposed = false
  // Guards against a slow response for an old query landing after a newer one.
  let generation = 0

  function cancelPending(): void {
    if (searchTimer) {
      clearTimeout(searchTimer)
      searchTimer = null
    }
    abort?.abort()
    abort = null
  }

  async function runSearch(query: string, gen: number): Promise<void> {
    abort = new AbortController()
    state.loading = true
    try {
      const resp = await searchEntities(query, undefined, abort.signal)
      if (disposed || gen !== generation) return
      state.items = rankByIdMatch(resp.data, query).slice(0, MAX_RESULTS)
      state.errorMsg = ''
      state.highlightedIndex = 0
    } catch (err: unknown) {
      if ((err as { name?: string })?.name === 'AbortError') return
      if (disposed || gen !== generation) return
      state.items = []
      state.errorMsg = 'Search failed'
    } finally {
      if (!disposed && gen === generation) state.loading = false
    }
  }

  function setQuery(query: string): void {
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
      return
    }
    searchTimer = setTimeout(() => {
      searchTimer = null
      void runSearch(query, gen)
    }, SEARCH_DEBOUNCE_MS)
  }

  function close(): void {
    cancelPending()
    // Bump the generation so an in-flight response cannot reopen the menu.
    generation++
    state.open = false
    state.query = ''
    state.items = []
    state.highlightedIndex = 0
    state.loading = false
    state.errorMsg = ''
  }

  function moveHighlight(delta: number): void {
    if (state.items.length === 0) return
    const n = state.items.length
    state.highlightedIndex = (state.highlightedIndex + delta + n) % n
  }

  function setHighlight(index: number): void {
    if (index < 0 || index >= state.items.length) return
    state.highlightedIndex = index
  }

  function current(): Entity | null {
    return state.items[state.highlightedIndex] ?? null
  }

  function dispose(): void {
    disposed = true
    cancelPending()
  }

  return {
    state: readonly(state) as DeepReadonly<MentionMenuState>,
    minQueryLength: MIN_SEARCH_LEN,
    setQuery,
    close,
    moveHighlight,
    setHighlight,
    current,
    dispose,
  }
}
