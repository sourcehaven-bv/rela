/**
 * The `@` completion menu for <rela-editor>, in plain DOM.
 *
 * This is the RENDERER and the TRANSPORT only. The state machine — when to
 * search, the debounce, the result cap, how a slow response is discarded —
 * is `mentionMenuState.ts`, shared with the SPA form, so the two menus cannot
 * come to disagree about any of it.
 *
 * Two things are genuinely local:
 *
 *   - Results come through the app bridge (`window.rela.search`), not axios. A
 *     sandboxed app has `connect-src 'none'` and cannot fetch on its own. The
 *     bridge has no abort signal, so the machine runs non-abortable and its
 *     generation guard is what resolves staleness.
 *   - Rows show the bare entity ID and no title. The bridge has no
 *     per-principal mentions endpoint, and deriving a title any other way would
 *     route around the read gate (BUG-R9EHKV). The search result the bridge
 *     returns is already scoped to what the principal may read, so listing IDs
 *     from it discloses nothing new, but the row must not claim more than the
 *     ID.
 */
import {
  createMentionMenuMachine,
  MIN_SEARCH_LEN,
  type MentionMenuMachine,
} from '@/components/forms/milkdown/mentionMenuState'
import { isValidEntityRefId } from '@/components/forms/milkdown/entityRefNode'

/** The one bridge method this module needs. */
export interface MentionSearchBridge {
  search(p: { query: string; type?: string }): Promise<{ data?: Array<{ id?: unknown }> }>
}

export interface MentionMenuHandle {
  /** The menu's root element, which the caller positions. */
  readonly root: HTMLElement
  readonly isOpen: boolean
  /** Opens the menu, or updates the query if it is already open. */
  setQuery(query: string): void
  close(): void
  moveHighlight(delta: number): void
  /** The ID under the highlight, or null when there is nothing to pick. */
  current(): string | null
  destroy(): void
}

export interface MentionMenuCallbacks {
  /** The user picked an entity ID. */
  pick(id: string): void
}

/** A menu row. Only the ID is ever held, let alone shown. */
interface MentionRow {
  id: string
}

export function createMentionMenu(
  bridge: MentionSearchBridge,
  cb: MentionMenuCallbacks
): MentionMenuHandle {
  const root = document.createElement('div')
  root.className = 'rela-mention-menu'
  root.setAttribute('role', 'listbox')

  let destroyed = false

  function note(text: string): void {
    const el = document.createElement('div')
    el.className = 'rela-mention-note'
    el.textContent = text
    root.appendChild(el)
  }

  function render(): void {
    const s = machine.state
    root.replaceChildren()
    if (!s.open) return
    if (s.query.length < MIN_SEARCH_LEN) {
      note(`Type ${MIN_SEARCH_LEN} characters to search`)
      return
    }
    if (s.loading) {
      note('Searching…')
      return
    }
    if (s.errorMsg) {
      note(s.errorMsg)
      return
    }
    if (s.items.length === 0) {
      note('No matches')
      return
    }
    s.items.forEach((row, i) => {
      const el = document.createElement('div')
      const active = i === s.highlightedIndex
      el.className = active ? 'rela-mention-option is-active' : 'rela-mention-option'
      el.setAttribute('role', 'option')
      el.setAttribute('aria-selected', String(active))
      // The bare ID, deliberately: see the module comment.
      el.textContent = row.id
      el.addEventListener('mousedown', (e) => e.preventDefault())
      el.addEventListener('click', () => cb.pick(row.id))
      el.addEventListener('mouseenter', () => machine.setHighlight(i))
      root.appendChild(el)
    })
  }

  const machine: MentionMenuMachine<MentionRow> = createMentionMenuMachine<MentionRow>({
    // The bridge cannot cancel a call, so staleness is resolved by the
    // machine's generation guard rather than by aborting.
    abortable: false,
    search: async (query) => {
      const resp = await bridge.search({ query })
      const rows: MentionRow[] = []
      for (const row of resp?.data ?? []) {
        // A row whose ID the editor cannot write as a reference is not offered:
        // picking it would insert nothing.
        if (isValidEntityRefId(row?.id)) rows.push({ id: row.id })
      }
      return rows
    },
    onChange: () => {
      if (!destroyed) render()
    },
  })

  return {
    root,
    get isOpen() {
      return machine.state.open
    },
    setQuery: (q) => machine.setQuery(q),
    close: () => machine.close(),
    moveHighlight: (d) => machine.moveHighlight(d),
    current: () => machine.current()?.id ?? null,
    destroy() {
      destroyed = true
      machine.dispose()
      root.remove()
    },
  }
}
