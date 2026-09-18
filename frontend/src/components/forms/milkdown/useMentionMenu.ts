/**
 * The `@` completion menu, as the SPA form consumes it.
 *
 * The old backtick flow made the user pick a type prefix first, then an entity,
 * because a backtick carries no information about what is wanted. The `@`
 * trigger drops that step: it searches every type at once.
 *
 * This is a thin Vue wrapper now. The state machine lives in
 * `mentionMenuState.ts` because the sandboxed app editor runs the same menu
 * without Vue, and a second copy of the search/staleness logic is exactly the
 * drift TKT-D2JML7 removed. What stays here is the reactivity and the axios
 * transport; nothing here touches ProseMirror.
 */
import { reactive, readonly, type DeepReadonly } from 'vue'
import { searchEntities } from '@/api'
import type { Entity } from '@/types'
import {
  createMentionMenuMachine,
  MIN_SEARCH_LEN,
  type MentionMenuSnapshot,
} from './mentionMenuState'

export type MentionMenuState = MentionMenuSnapshot<Entity>

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

export function useMentionMenu(): MentionMenuController {
  // The machine mutates its snapshot in place, so wrapping it in `reactive`
  // makes every mutation reach the template without the machine knowing Vue
  // exists.
  const state = reactive<MentionMenuState>({
    open: false,
    query: '',
    items: [],
    highlightedIndex: 0,
    loading: false,
    errorMsg: '',
  })

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
  }

  const machine = createMentionMenuMachine<Entity>({
    // axios can cancel, so the signal is passed through and an abort is not
    // reported to the user as a failed search.
    abortable: true,
    search: async (query, signal) => {
      const resp = await searchEntities(query, undefined, signal)
      return resp.data
    },
    onChange: () => sync(),
  })

  return {
    state: readonly(state) as DeepReadonly<MentionMenuState>,
    minQueryLength: MIN_SEARCH_LEN,
    setQuery: (q) => machine.setQuery(q),
    close: () => machine.close(),
    moveHighlight: (d) => machine.moveHighlight(d),
    setHighlight: (i) => machine.setHighlight(i),
    current: () => machine.current(),
    dispose: () => machine.dispose(),
  }
}
