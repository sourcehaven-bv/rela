/**
 * The `@` completion menu, as the SPA form consumes it.
 *
 * The old backtick flow made the user pick a type prefix first, then an entity,
 * because a backtick carries no information about what is wanted. `@` searches
 * every type at once, and this adds an *optional* type picker on top: types are
 * offered, never required. Picking one scopes the search through the `type`
 * parameter `searchEntities` already takes.
 *
 * The search/staleness state machine lives in `mentionMenuState.ts` because the
 * sandboxed app editor runs the same menu without Vue, and a second copy of that
 * logic is exactly the drift TKT-D2JML7 removed. What stays here is the
 * reactivity, the axios transport, and the **type picker**, which is SPA-only:
 * the sandboxed app runs under `connect-src 'none'` and has no route to the
 * schema's type list, so giving it one is a separate decision (not made). The
 * fuzzy ranking DOES live in the shared `rankMentions.ts`, so both editors get
 * it. Nothing here touches ProseMirror.
 *
 * # The query reaches the server unmodified
 *
 * Nothing here rewrites, splits or sanitizes the query. Sending word-split forms
 * was measured to destroy ID ranking on the bleve backend (the regression
 * BUG-O09QUC exists to prevent) and to return nothing at all on the linear and
 * postgres backends, which match the query as one substring. Cross-field reach
 * comes from the type picker, not from rewriting what the user typed. It also
 * keeps the query a single token, so filter syntax such as `@status:open` stays
 * inert.
 */
import { reactive, readonly, watch, type DeepReadonly } from 'vue'
import { searchEntities } from '@/api'
import { useSchemaStore } from '@/stores/schema'
import type { Entity } from '@/types'
import {
  createMentionMenuMachine,
  MIN_SEARCH_LEN,
  type MentionMenuSnapshot,
} from './mentionMenuState'
import { rankTypeNames } from './rankMentions'

/** Type suggestions shown alongside entity results once a search is running. */
const MAX_TYPE_SUGGESTIONS = 3
/**
 * Above this query length the types section is hidden entirely.
 *
 * A long query is a statement that the user is naming an entity, not browsing
 * for a type, and the types section costs a row of the menu either way.
 */
const TYPE_SECTION_MAX_QUERY = 6

/** What the highlight currently sits on. Enter means different things per kind. */
export type MentionChoice = { kind: 'type'; name: string } | { kind: 'entity'; entity: Entity }

/**
 * WHAT the highlight is on, rather than where it sat.
 *
 * The rows are recomputed asynchronously under the user: the type suggestions
 * re-rank on every keystroke (and vanish past `TYPE_SECTION_MAX_QUERY`), while
 * the entities arrive a debounce later. A stored INDEX addressing that is
 * unsound, and was wrong four different ways — it could point past the end
 * (making Enter a no-op that fell through to ProseMirror and inserted a
 * paragraph break), or slide onto an unrelated row when the list above it
 * shrank, so Enter inserted an entity the user never highlighted.
 *
 * Storing the identity makes those unrepresentable: it either still resolves to
 * the same row or it does not resolve at all, and `highlightedIndex()` falls
 * back to the first row rather than inventing a selection.
 */
export type MentionHighlight =
  | { kind: 'type'; name: string }
  | { kind: 'entity'; id: string }
  | null

/**
 * The SPA menu's state: the shared snapshot plus the type-picker dimension.
 *
 * The extra fields live here rather than in `MentionMenuSnapshot` because the
 * app editor's menu has no type picker; widening the shared snapshot would make
 * every plain-DOM renderer carry fields it can never populate.
 */
export interface MentionMenuState extends MentionMenuSnapshot<Entity> {
  /** Type names offered for scoping, already ranked and capped. */
  typeItems: string[]
  /** The type the search is scoped to, or null for every type. */
  selectedType: string | null
  /**
   * The highlighted row's identity, or null to mean "the first row".
   *
   * Read `highlightedIndex()` for rendering; this is the source of truth. The
   * shared snapshot's numeric `highlightedIndex` addresses the ENTITY list only
   * and is left to the machine.
   */
  highlight: MentionHighlight
}

export interface MentionMenuController {
  state: DeepReadonly<MentionMenuState>
  /** The shortest query that triggers a search; below it the menu prompts. */
  minQueryLength: number
  /** Opens the menu, or updates the query if it is already open. */
  setQuery: (query: string) => void
  /** Scopes the search to a type and re-runs it against the current query. */
  selectType: (name: string) => void
  /** Removes the scope and re-runs the search across every type. */
  clearType: () => void
  /** Supplies the type names to offer; the caller owns the schema store. */
  setAvailableTypes: (names: string[]) => void
  close: () => void
  moveHighlight: (delta: number) => void
  /** Highlights the row at this position in the combined list. */
  setHighlight: (index: number) => void
  /**
   * Where the highlight currently resolves to in the combined list.
   *
   * Derived from the stored identity on every call rather than kept as state,
   * so it cannot go stale against rows that changed underneath it. Falls back
   * to 0 when the highlighted row is gone, and is -1 when there are no rows.
   */
  highlightedIndex: () => number
  /** What the highlight sits on, or null when there is nothing to pick. */
  current: () => MentionChoice | null
  dispose: () => void
}

/**
 * A mention menu with the type picker bound to the live schema.
 *
 * Separate from [useMentionMenu] so the composable itself stays free of store
 * context: its tests supply types through `setAvailableTypes` and need no Pinia
 * instance. This wrapper is what a component uses.
 *
 * The list is watched rather than read once because the schema loads on app
 * mount and an editor can mount before that resolves. Type names are not
 * confidential (root CLAUDE.md: "The configuration is not a secret"), so every
 * principal is offered the full list and no per-principal filtering applies.
 */
export function useSchemaMentionMenu(): MentionMenuController {
  const menu = useMentionMenu()
  const schemaStore = useSchemaStore()
  watch(
    () => schemaStore.entityTypeList.map(([name]) => name),
    (names) => menu.setAvailableTypes(names),
    { immediate: true }
  )
  return menu
}

/** True when two name lists are equal element-wise. */
function sameNames(a: readonly string[], b: readonly string[]): boolean {
  return a.length === b.length && a.every((n, i) => n === b[i])
}

export function useMentionMenu(): MentionMenuController {
  // The machine mutates its snapshot in place, so wrapping it in `reactive`
  // makes every mutation reach the template without the machine knowing Vue
  // exists. The type-picker fields are ours; the machine never touches them.
  const state = reactive<MentionMenuState>({
    open: false,
    query: '',
    items: [],
    highlightedIndex: 0,
    loading: false,
    errorMsg: '',
    typeItems: [],
    selectedType: null,
    highlight: null,
  })

  /** Every type name the schema knows, unranked. The allowlist for `?type=`. */
  let availableTypes: string[] = []
  let disposed = false

  // Project the machine's snapshot onto the reactive object. Assigning the
  // fields rather than swapping the object keeps a `readonly(state)` handle a
  // component already holds pointing at the right thing.
  //
  // Driven by the machine's own `onChange` rather than by wrapping each method:
  // the interesting transitions are ASYNCHRONOUS (a search resolving, a stale
  // response being discarded), so a wrapper that synced on the synchronous call
  // would show an empty list forever.
  const sync = (): void => {
    Object.assign(state, machine.state)
    // A fresh result set invalidates an entity highlight that is no longer in
    // it. The machine owns `items`, so this is the point where that is known.
    if (state.highlight?.kind === 'entity') {
      const id = state.highlight.id
      if (!state.items.some((e) => e.id === id)) state.highlight = null
    }
    refreshTypeItems()
  }

  const machine = createMentionMenuMachine<Entity>({
    // axios can cancel, so the signal is passed through and an abort is not
    // reported to the user as a failed search.
    abortable: true,
    search: async (query, signal) => {
      // `selectedType ?? undefined`: the API treats undefined as "every type",
      // and a null would serialize into the query string. Read at call time so
      // a scope change searches the new scope.
      const resp = await searchEntities(query, state.selectedType ?? undefined, signal)
      return resp.data
    },
    onChange: () => sync(),
  })

  /**
   * Recomputes the type suggestions for the current query.
   *
   * Hidden outright once a type is picked (the scope is shown as a chip instead,
   * and offering a second type would suggest the two combine) or once the query
   * grows past `TYPE_SECTION_MAX_QUERY`.
   *
   * Only a BARE `@` lists everything. One letter is already enough to narrow a
   * couple of dozen types to a handful, so the type list is deliberately NOT
   * tied to `MIN_SEARCH_LEN` — that threshold exists to stop a one-character
   * ENTITY search reaching the server, which is a different concern.
   */
  function refreshTypeItems(): void {
    // A closed menu offers nothing. Without this a schema reload repopulates the
    // list behind a closed menu, making `close()` a liar and leaving a later
    // `v-if` on typeItems.length able to pop the menu open by itself.
    if (!state.open) {
      state.typeItems = []
      return
    }
    if (state.selectedType !== null || state.query.length > TYPE_SECTION_MAX_QUERY) {
      state.typeItems = []
      return
    }
    if (state.query.length === 0) {
      state.typeItems = availableTypes
      return
    }
    state.typeItems = rankTypeNames(availableTypes, state.query).slice(0, MAX_TYPE_SUGGESTIONS)
  }

  function setQuery(query: string): void {
    if (disposed) return
    machine.setQuery(query)
    // The machine's `onChange` covers the async transitions, but a query below
    // the search minimum settles synchronously, so the type section is
    // refreshed here as well as in `sync`.
    state.open = true
    state.query = query
    refreshTypeItems()
  }

  function selectType(name: string): void {
    if (disposed) return
    // Allowlist by construction: only a name the schema supplied can become a
    // `?type=` value, never free-typed text.
    if (!availableTypes.includes(name)) return
    if (state.selectedType === name) return
    state.selectedType = name
    applyScopeChange()
  }

  function clearType(): void {
    if (disposed) return
    if (state.selectedType === null) return
    state.selectedType = null
    applyScopeChange()
  }

  /**
   * Re-runs the search after the scope changed, dropping what the old scope
   * answered.
   *
   * The rows on screen answered a different question, and the chip already
   * claims the new scope — leaving them would show entities of the wrong type
   * under a type's name, and an Enter in that window would insert one.
   */
  function applyScopeChange(): void {
    // Re-issue the current query FIRST; the machine re-reads
    // `state.selectedType` inside its `search` closure.
    //
    // Order matters. `machine.setQuery` calls back into `sync()`, which
    // `Object.assign`s the machine's snapshot over ours — so clearing `items`
    // before this call would be silently undone by that copy, leaving the old
    // scope's rows under the new chip. The machine keeps its previous results
    // until the debounced search resolves, so the clear has to land after.
    machine.setQuery(state.query)
    state.items = []
    state.highlight = null
    refreshTypeItems()
  }

  function setAvailableTypes(names: string[]): void {
    if (disposed) return
    if (sameNames(availableTypes, names)) return
    availableTypes = names
    // A schema reload can retire the scoped type; dropping the stale scope is
    // safer than searching a type that no longer exists.
    if (state.selectedType !== null && !names.includes(state.selectedType)) {
      state.selectedType = null
    }
    refreshTypeItems()
  }

  function close(): void {
    machine.close()
    state.open = false
    state.typeItems = []
    state.selectedType = null
    state.highlight = null
  }

  /** Total rows the highlight can address: types first, then entities. */
  function choiceCount(): number {
    return state.typeItems.length + state.items.length
  }

  /** The row at `index` in the combined list, types first. */
  function choiceAt(index: number): MentionChoice | null {
    const typeCount = state.typeItems.length
    if (index < 0) return null
    if (index < typeCount) {
      const name = state.typeItems[index]
      return name === undefined ? null : { kind: 'type', name }
    }
    const entity = state.items[index - typeCount]
    return entity === undefined ? null : { kind: 'entity', entity: entity as Entity }
  }

  /**
   * Where the stored identity currently sits, or 0 when it no longer resolves.
   *
   * Resolving on read is what makes a changing list safe: a highlight whose row
   * is gone falls back to the first row instead of addressing whatever moved
   * into its old position.
   */
  function highlightedIndex(): number {
    const n = choiceCount()
    if (n === 0) return -1
    const h = state.highlight
    if (h !== null) {
      if (h.kind === 'type') {
        const at = state.typeItems.indexOf(h.name)
        if (at !== -1) return at
      } else {
        const at = state.items.findIndex((e) => e.id === h.id)
        if (at !== -1) return state.typeItems.length + at
      }
    }
    return 0
  }

  function moveHighlight(delta: number): void {
    const n = choiceCount()
    if (n === 0) return
    setHighlight((highlightedIndex() + delta + n) % n)
  }

  function setHighlight(index: number): void {
    const choice = choiceAt(index)
    if (choice === null) return
    state.highlight =
      choice.kind === 'type'
        ? { kind: 'type', name: choice.name }
        : { kind: 'entity', id: choice.entity.id }
  }

  function current(): MentionChoice | null {
    return choiceAt(highlightedIndex())
  }

  function dispose(): void {
    disposed = true
    machine.dispose()
  }

  return {
    state: readonly(state) as DeepReadonly<MentionMenuState>,
    minQueryLength: MIN_SEARCH_LEN,
    setQuery,
    selectType,
    clearType,
    setAvailableTypes,
    close,
    moveHighlight,
    setHighlight,
    highlightedIndex,
    current,
    dispose,
  }
}
