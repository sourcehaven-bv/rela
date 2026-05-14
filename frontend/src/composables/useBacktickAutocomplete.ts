// Inline backtick-triggered entity-reference autocomplete for the
// MarkdownEditor (TKT-2RCP). Subscribes to CodeMirror events to drive a
// non-focus-stealing popup that walks the user through prefix → id
// selection without ever moving document focus off the editor.
//
// Architecture:
//   - state machine: idle → pending → prefix → id → idle
//   - structural-context detection via the two-side `getTokenAt` rule
//     validated against the EasyMDE prototype (see TKT-2RCP)
//   - open-delay grace period (600 ms default) so literal code spans
//     never flash a popup
//   - phase 1 filters a project-derived prefix list (no API call); phase
//     2 debounces `searchEntities(q, type)` with abort-on-close
//   - inserts via TKT-I5NO's `insertEntityRef` helper so adjacency
//     padding, denylist validation, and null-editor safety are shared
import { reactive, readonly, type DeepReadonly } from 'vue'
import type EasyMDE from 'easymde'
import { searchEntities, listEntities } from '@/api'
import type { Entity, EntityType } from '@/types'
import { insertEntityRef } from '@/components/forms/insertEntityRef'

/** Open-delay grace period before the popup is shown. Tuned via the
 *  static prototype: 600 ms feels right — fast typists never see the
 *  popup for literal code spans like `` `flag-name` ``, deliberate
 *  references trigger reliably. */
export const OPEN_DELAY_MS = 600

/** Debounce for the phase-2 search query to /_search. Matches the
 *  CommandPaletteModal / EntityPickerModal cadence. */
const SEARCH_DEBOUNCE_MS = 150

/** Cap results client-side. Mirrors EntityPickerModal. */
const MAX_RESULTS = 50

/** Minimum partial-query length before phase 2 calls /_search. Below
 *  this, the popup shows whatever the resolver returns for an empty
 *  query (typically the most-recent entities of the type). */
const MIN_SEARCH_LEN = 1

/** Characters that, when typed, immediately dismiss the session.
 *  Anything that can't be part of an entity ID's left-to-right scan
 *  ends the trigger window. */
const NON_ID_CHAR = /[\s`(){}[\]<>,;:!?"'\\]/

export type Phase = 'idle' | 'pending' | 'prefix' | 'id'

export interface PrefixItem {
  /** The literal prefix text, e.g. `"TKT-"`. For manual-id types this
   *  is empty — selecting the row jumps straight to phase 2 with the
   *  type filter, without inserting any prefix text. */
  prefix: string
  /** Entity type machine name, used for the phase-2 search filter. */
  type: string
  /** Human-readable label from the metamodel (`entity_def.label`). */
  label: string
  /** True when the type's id_type is `'manual'` and has no prefix.
   *  Phase 1 shows these with a "(manual)" hint instead of a "TKT-*"-
   *  style preview. */
  isManual: boolean
}

export interface SessionState {
  phase: Phase
  /** Trigger backtick position. Null when idle. */
  triggerPos: { line: number; ch: number } | null
  /** Phase-1 results — filtered list of project prefixes. */
  prefixItems: PrefixItem[]
  /** Phase-2 results — entities of the resolved type. */
  entityItems: Entity[]
  /** Highlighted row in whichever list is showing. */
  highlightedIndex: number
  /** Set when the resolver picked a prefix and we're now in phase 2. */
  resolvedPrefix: PrefixItem | null
  /** Phase-2 search transport state. */
  loading: boolean
  errorMsg: string
  /** Pixel coords of the trigger (for popup placement). Provided by
   *  CodeMirror's `charCoords` so callers don't have to know about the
   *  editor's DOM. */
  anchor: { left: number; top: number; bottom: number } | null
}

/** Read-only view of `SessionState` exposed to consumers. The composable
 *  owns the only mutating path; consumers may only inspect. */
export type ReadonlySessionState = DeepReadonly<SessionState>

/** Minimal slice of EasyMDE that the composable touches. Typed as a
 *  structural interface so unit tests can pass a hand-rolled shim with
 *  fake event emitters and a synchronous `getTokenAt`. */
export interface EasyMdeLike {
  codemirror: CodeMirrorLike
}

interface CodeMirrorPos {
  line: number
  ch: number
}

interface CodeMirrorToken {
  type: string | null
  string: string
}

interface CodeMirrorInputReadChange {
  text: string[]
  from: CodeMirrorPos
}

/** Per-event handler signatures we subscribe to. Mirrors CodeMirror v5's
 *  documented event shapes; using a discriminated overload set means
 *  `cm.on('inputRead', fn)` type-checks against `fn`'s shape without
 *  the `as never` escape hatches the earlier version used (RR-OCSO). */
interface CodeMirrorLike {
  getCursor: (side?: 'from' | 'to' | 'anchor' | 'head') => CodeMirrorPos
  getTokenAt: (pos: CodeMirrorPos, precise?: boolean) => CodeMirrorToken | null
  getRange: (from: CodeMirrorPos, to: CodeMirrorPos) => string
  replaceRange: (text: string, from: CodeMirrorPos, to?: CodeMirrorPos) => void
  charCoords: (
    pos: CodeMirrorPos,
    mode?: 'window' | 'page' | 'local',
  ) => { left: number; top: number; bottom: number; right: number }
  on(
    event: 'inputRead',
    handler: (cm: CodeMirrorLike, change: CodeMirrorInputReadChange) => void,
  ): void
  on(event: 'change', handler: () => void): void
  on(event: 'cursorActivity', handler: () => void): void
  on(event: 'blur', handler: () => void): void
  on(event: 'keydown', handler: (cm: CodeMirrorLike, e: KeyboardEvent) => void): void
  off(
    event: 'inputRead',
    handler: (cm: CodeMirrorLike, change: CodeMirrorInputReadChange) => void,
  ): void
  off(event: 'change', handler: () => void): void
  off(event: 'cursorActivity', handler: () => void): void
  off(event: 'blur', handler: () => void): void
  off(event: 'keydown', handler: (cm: CodeMirrorLike, e: KeyboardEvent) => void): void
}

/** Source of entity-type definitions. Decoupled from `schemaStore` so
 *  unit tests can hand-build small fixtures. Production callers pass
 *  `() => schemaStore.entityTypes`. */
export type EntityTypesProvider = () => Map<string, EntityType>

export interface SessionController {
  state: ReadonlySessionState
  /** Apply the highlighted row. Phase 1 picks a prefix → transitions to
   *  phase 2. Phase 2 inserts `` `<id>` `` via insertEntityRef and
   *  closes. */
  pick: () => void
  /** Move the highlight up/down with wrap-around. */
  moveHighlight: (delta: -1 | 1) => void
  /** Set the highlight to a specific index (used by mouse hover/click
   *  in the popup). Out-of-range values are clamped. */
  setHighlight: (index: number) => void
  /** Force-close the session without inserting. */
  dismiss: () => void
  /** Tear down all CodeMirror subscriptions. MarkdownEditor calls this
   *  in onBeforeUnmount before nulling the editor. */
  dispose: () => void
}

/** Token types CodeMirror's markdown mode attaches to characters that
 *  must NOT start a fresh inline code span. */
const SUPPRESS_TYPES = ['comment', 'code', 'link', 'url', 'string', 'meta', 'tag']

function isInlineTextContext(tok: { type: string | null } | null): boolean {
  if (!tok || !tok.type) return true
  for (const bad of SUPPRESS_TYPES) {
    if (tok.type.indexOf(bad) !== -1) return false
  }
  return true
}

function hasFormatting(tok: { type: string | null } | null): boolean {
  return !!(tok && tok.type && tok.type.indexOf('formatting') !== -1)
}

/** Build the prefix list for phase 1. Each entity type contributes:
 *  - one entry per declared `id_prefix` / `id_prefixes` value, OR
 *  - a single "manual" entry when `id_type` is `'manual'` (no prefix).
 *  Sorted alphabetically by prefix (manual entries collated by label). */
export function buildPrefixList(entityTypes: Map<string, EntityType>): PrefixItem[] {
  const items: PrefixItem[] = []
  for (const [typeName, def] of entityTypes.entries()) {
    // The backend serves both `id_prefix` and `id_prefixes` for short-id
    // types (the latter mirrors the former as a single-element array), so
    // a plain merge would surface every prefix twice. Dedupe per type.
    const seen = new Set<string>()
    const prefixes: string[] = []
    const push = (p: string | undefined): void => {
      if (!p || seen.has(p)) return
      seen.add(p)
      prefixes.push(p)
    }
    push(def.id_prefix)
    if (def.id_prefixes) def.id_prefixes.forEach(push)
    if (prefixes.length === 0 && def.id_type === 'manual') {
      items.push({ prefix: '', type: typeName, label: def.label, isManual: true })
      continue
    }
    for (const prefix of prefixes) {
      items.push({ prefix, type: typeName, label: def.label, isManual: false })
    }
  }
  items.sort((a, b) => {
    if (a.prefix === '' && b.prefix !== '') return 1
    if (a.prefix !== '' && b.prefix === '') return -1
    if (a.prefix === b.prefix) return a.label.localeCompare(b.label)
    return a.prefix.localeCompare(b.prefix)
  })
  return items
}

/** Driver. Wires the session to a live EasyMDE instance and returns a
 *  controller the parent component (popup + MarkdownEditor) consumes. */
export function useBacktickAutocomplete(
  editor: EasyMdeLike,
  entityTypes: EntityTypesProvider,
  options: { openDelayMs?: number } = {},
): SessionController {
  const openDelay = options.openDelayMs ?? OPEN_DELAY_MS
  const cm = editor.codemirror

  const state = reactive<SessionState>({
    phase: 'idle',
    triggerPos: null,
    prefixItems: [],
    entityItems: [],
    highlightedIndex: 0,
    resolvedPrefix: null,
    loading: false,
    errorMsg: '',
    anchor: null,
  })

  let openTimer: ReturnType<typeof setTimeout> | null = null
  let searchTimer: ReturnType<typeof setTimeout> | null = null
  let abort: AbortController | null = null
  let allPrefixes: PrefixItem[] = []
  // Tracks the cursor position at the END of the typed-after-trigger
  // range after the most recent buffer change. cursorActivity closes
  // the session if the cursor moves outside `[trigger, expectedEnd]` —
  // mouse-clicking past the typed range (e.g. trigger at ch:5, typed
  // up to ch:10, click at ch:30) would otherwise leave a zombie
  // session feeding garbage into the filter (RR-E25Z).
  let expectedCursorCh = -1

  function cancelInflight(): void {
    if (openTimer) {
      clearTimeout(openTimer)
      openTimer = null
    }
    if (searchTimer) {
      clearTimeout(searchTimer)
      searchTimer = null
    }
    abort?.abort()
    abort = null
  }

  function resetSession(): void {
    cancelInflight()
    state.phase = 'idle'
    state.triggerPos = null
    state.prefixItems = []
    state.entityItems = []
    state.highlightedIndex = 0
    state.resolvedPrefix = null
    state.loading = false
    state.errorMsg = ''
    state.anchor = null
    expectedCursorCh = -1
  }

  function typedAfterTrigger(): string {
    const trig = state.triggerPos
    if (!trig) return ''
    const cursor = cm.getCursor()
    if (cursor.line !== trig.line || cursor.ch <= trig.ch) return ''
    return cm.getRange({ line: trig.line, ch: trig.ch + 1 }, cursor)
  }

  function placeAnchor(): void {
    if (!state.triggerPos) return
    const coords = cm.charCoords({ line: state.triggerPos.line, ch: state.triggerPos.ch }, 'window')
    state.anchor = { left: coords.left, top: coords.top, bottom: coords.bottom }
  }

  function transitionToPrefix(): void {
    state.phase = 'prefix'
    allPrefixes = buildPrefixList(entityTypes())
    if (allPrefixes.length === 0) {
      // Edge case: project has no entity types declared with prefixes
      // OR manual types. Nothing to offer.
      resetSession()
      return
    }
    state.prefixItems = allPrefixes
    state.highlightedIndex = 0
    placeAnchor()
  }

  function filterPrefixList(typed: string): void {
    const upper = typed.toUpperCase()
    const lower = typed.toLowerCase()
    state.prefixItems = allPrefixes.filter((p) => {
      if (p.isManual) {
        // Manual types match if the typed text is a case-insensitive
        // prefix of the type's label. `startsWith` (not `includes`) so
        // a manual type labeled `Code` doesn't surface for unrelated
        // typed text like `de` (RR-HUNK).
        return p.label.toLowerCase().startsWith(lower)
      }
      // Either side may be the longer string: while the user types
      // toward a prefix (typed is shorter than prefix → keep) and once
      // they've typed past the prefix into the id body (typed starts
      // with the prefix → also keep, the exact-match path will then
      // transition to phase 2).
      const pUpper = p.prefix.toUpperCase()
      return pUpper.startsWith(upper) || upper.startsWith(pUpper)
    })
    if (state.prefixItems.length === 0) {
      // No prefix matches — close. The user is typing something the
      // resolver doesn't recognize as a reference prefix.
      resetSession()
      return
    }
    if (state.highlightedIndex >= state.prefixItems.length) {
      state.highlightedIndex = state.prefixItems.length - 1
    }
  }

  function tryExactPrefixMatch(typed: string): PrefixItem | null {
    const upper = typed.toUpperCase()
    // Exact match first — covers `TKT-` style prefixes that already
    // include a trailing dash.
    const exact = state.prefixItems.find(
      (p) => !p.isManual && p.prefix.toUpperCase() === upper,
    )
    if (exact) return exact
    // Two "typed continues past the prefix" cases:
    //   (a) prefix includes a trailing `-` (`TKT-`) and typed has more
    //       chars: `typed.startsWith(prefix)`.
    //   (b) prefix lacks the trailing `-` (`FEAT`) and typed adds the
    //       conventional separator: `typed.startsWith(prefix + '-')`.
    // Both share the same shape: typed strictly starts with the prefix
    // AND has at least one id-body char past it.
    //
    // Disambiguation: if two prefixes both qualify (e.g. `PROJ` and
    // `PROJX` against typed `PROJX-…`), the LONGEST prefix wins. The
    // first-match alphabetical-sort behaviour the earlier code had was
    // a footgun (RR-L56D).
    let best: PrefixItem | null = null
    for (const p of state.prefixItems) {
      if (p.isManual || p.prefix.length === 0) continue
      const pUpper = p.prefix.toUpperCase()
      const startsWithPrefix = upper.startsWith(pUpper) && upper.length > pUpper.length
      const startsWithPrefixDash =
        !pUpper.endsWith('-') && upper.startsWith(pUpper + '-')
      if (!startsWithPrefix && !startsWithPrefixDash) continue
      if (best === null || p.prefix.length > best.prefix.length) {
        best = p
      }
    }
    return best
  }

  function transitionToIdPhase(prefix: PrefixItem): void {
    state.phase = 'id'
    state.resolvedPrefix = prefix
    state.entityItems = []
    state.highlightedIndex = 0
    state.errorMsg = ''
    scheduleSearch('')
  }

  function scheduleSearch(query: string): void {
    if (searchTimer) clearTimeout(searchTimer)
    abort?.abort()
    abort = null
    searchTimer = setTimeout(() => {
      void runSearch(query)
    }, SEARCH_DEBOUNCE_MS)
  }

  async function runSearch(query: string): Promise<void> {
    if (!state.resolvedPrefix) return
    abort = new AbortController()
    state.loading = true
    try {
      // Empty/very-short queries call the listing endpoint instead of
      // /_search?q=*. The wildcard goes through Bleve's full TF-IDF
      // pipeline and returns relevance-scored junk; the listing
      // endpoint returns the type's configured default sort which is
      // what the user wants when they haven't typed anything narrowing
      // yet (RR-UNAK).
      const type = state.resolvedPrefix.type
      const resp =
        query.length >= MIN_SEARCH_LEN
          ? await searchEntities(query, type, abort.signal)
          : await listEntities(type, { per_page: MAX_RESULTS })
      state.entityItems = resp.data.slice(0, MAX_RESULTS)
      state.errorMsg = ''
      state.highlightedIndex = 0
    } catch (err: unknown) {
      // Aborted requests are not errors.
      if ((err as { name?: string })?.name === 'AbortError') return
      state.errorMsg = 'Search failed'
    } finally {
      state.loading = false
    }
  }

  /* Event handlers ----------------------------------------------------- */

  function onInputRead(_: CodeMirrorLike, change: CodeMirrorInputReadChange): void {
    if (state.phase !== 'idle') return
    if (change.text.length !== 1) return
    if (change.text[0] !== '`') return

    // Two-side classification — see TKT-2RCP for the prototype that
    // validated this rule against fenced blocks, link URLs, and
    // closing-backtick cases.
    const triggerLine = change.from.line
    const triggerCh = change.from.ch
    const tokAfter = cm.getTokenAt({ line: triggerLine, ch: triggerCh + 1 }, true)
    // tokBefore is undefined when the trigger sits at ch:0 (no
    // character to the left). Start-of-line counts as inline-text
    // context for the open-decision purpose — there's no code-span
    // body that we could be closing.
    const tokBefore =
      triggerCh > 0
        ? cm.getTokenAt({ line: triggerLine, ch: triggerCh - 1 }, true)
        : null

    if (!hasFormatting(tokAfter)) return
    if (tokBefore && !isInlineTextContext(tokBefore)) return

    state.triggerPos = { line: triggerLine, ch: triggerCh }
    state.phase = 'pending'
    // Cursor sits one past the backtick after insertion.
    expectedCursorCh = triggerCh + 1
    placeAnchor()

    openTimer = setTimeout(() => {
      openTimer = null
      // Re-check that the user hasn't typed a disqualifying character
      // during the delay window.
      if (state.phase !== 'pending') return
      const typed = typedAfterTrigger()
      if (NON_ID_CHAR.test(typed)) {
        resetSession()
        return
      }
      transitionToPrefix()
      // Apply any prefix typed during the delay window — without this,
      // a user who types `\`FEAT-` quickly would land in phase 1 with
      // the full prefix list, instead of phase 2 with the resolved
      // type already filtered. `applyTypedToPhase` handles the same
      // logic the change-handler runs and is the single source of
      // truth for "typed text → phase transition" so the timer
      // doesn't open a parallel path (RR-RH10).
      if (typed.length > 0) {
        applyTypedToPhase(typed)
      }
    }, openDelay)
  }

  /** Re-evaluate the phase machine given the current typed-after-trigger
   *  text. Single source of truth for "the user typed something; now
   *  what does the popup show?" — called both from the open-delay timer
   *  (to handle prefix text typed while the popup was pending) and from
   *  the `change` event handler. Idempotent: safe to call when phase is
   *  idle (no-op). */
  function applyTypedToPhase(typed: string): void {
    if (state.phase === 'prefix') {
      filterPrefixList(typed)
      if (state.phase !== 'prefix') return // filterPrefixList may have closed us
      const exact = tryExactPrefixMatch(typed)
      if (exact) {
        transitionToIdPhase(exact)
      }
    }
  }

  function onChange(): void {
    if (state.phase === 'idle') return
    const cursor = cm.getCursor()
    const trig = state.triggerPos
    if (!trig) return
    // Backspaced past the trigger? Close.
    if (cursor.line !== trig.line || cursor.ch <= trig.ch) {
      resetSession()
      return
    }
    const typed = typedAfterTrigger()
    if (NON_ID_CHAR.test(typed)) {
      resetSession()
      return
    }
    // Cursor is at the end of the typed-after-trigger range — record
    // it so cursorActivity can spot mouse-driven jumps past this point.
    expectedCursorCh = cursor.ch
    if (state.phase === 'pending') return // wait for the delay

    applyTypedToPhase(typed)
    if (state.phase === 'id' as Phase) {
      // In phase 2, characters past the prefix narrow the entity-id
      // query. The metamodel records the prefix two ways: either with
      // the trailing `-` (e.g. `TKT-`) or without (`FEAT` if the
      // project author dropped the dash). The conventional `<PREFIX>-<id>`
      // shape means the typed text always uses a `-` as the separator,
      // so we strip that too when slicing the partial id query.
      const prefix = state.resolvedPrefix?.prefix ?? ''
      let partial = typed.toUpperCase().startsWith(prefix.toUpperCase())
        ? typed.slice(prefix.length)
        : typed
      if (partial.startsWith('-')) partial = partial.slice(1)
      scheduleSearch(partial)
    }
  }

  function onCursorActivity(): void {
    if (state.phase === 'idle') return
    const cursor = cm.getCursor()
    const trig = state.triggerPos
    if (!trig) return
    if (cursor.line !== trig.line || cursor.ch <= trig.ch) {
      resetSession()
      return
    }
    // RR-E25Z: clicking past the typed-after-trigger range (e.g. moving
    // the cursor far to the right of where typing left it) closes the
    // session. We let the cursor move by +1 from the last typing point
    // because change/cursorActivity fire in either order on a keystroke
    // and the cursor may end up one position ahead of `expectedCursorCh`
    // when cursorActivity races change.
    if (expectedCursorCh >= 0 && cursor.ch > expectedCursorCh + 1) {
      resetSession()
    }
  }

  function onBlur(): void {
    resetSession()
  }

  function onKeyDown(_: CodeMirrorLike, e: KeyboardEvent): void {
    if (state.phase !== 'prefix' && state.phase !== 'id') {
      // During 'pending' we still respond to Escape so the user can
      // cancel before the delay elapses.
      if (state.phase === 'pending' && e.key === 'Escape') {
        e.preventDefault()
        resetSession()
      }
      return
    }
    switch (e.key) {
      case 'Escape':
        e.preventDefault()
        // Use the EasyMDE method on the event for compatibility with
        // CodeMirror v5's event wrapper. `stopPropagation` keeps the
        // global keyboard-shortcut Escape branch from firing too.
        if (typeof (e as KeyboardEvent & { stopPropagation?: () => void }).stopPropagation === 'function') {
          e.stopPropagation()
        }
        resetSession()
        return
      case 'Enter':
        e.preventDefault()
        pick()
        return
      case 'ArrowDown':
        e.preventDefault()
        moveHighlight(1)
        return
      case 'ArrowUp':
        e.preventDefault()
        moveHighlight(-1)
        return
    }
  }

  /* Public API --------------------------------------------------------- */

  function moveHighlight(delta: -1 | 1): void {
    const list = state.phase === 'prefix' ? state.prefixItems : state.entityItems
    if (list.length === 0) {
      state.highlightedIndex = 0
      return
    }
    state.highlightedIndex = (state.highlightedIndex + delta + list.length) % list.length
  }

  function setHighlight(index: number): void {
    const list = state.phase === 'prefix' ? state.prefixItems : state.entityItems
    if (list.length === 0) {
      state.highlightedIndex = 0
      return
    }
    if (index < 0) {
      state.highlightedIndex = 0
    } else if (index >= list.length) {
      state.highlightedIndex = list.length - 1
    } else {
      state.highlightedIndex = index
    }
  }

  function pick(): void {
    if (state.phase === 'prefix') {
      const item = state.prefixItems[state.highlightedIndex]
      if (!item) return
      if (item.isManual) {
        // Manual: no prefix to insert, jump straight to phase 2 with
        // the type filter.
        transitionToIdPhase(item)
        return
      }
      // Insert the prefix into the buffer at the trigger position, then
      // transition to phase 2.
      const trig = state.triggerPos
      if (!trig) return
      const cursor = cm.getCursor()
      cm.replaceRange(item.prefix, { line: trig.line, ch: trig.ch + 1 }, cursor)
      transitionToIdPhase(item)
      return
    }
    if (state.phase === 'id') {
      const item = state.entityItems[state.highlightedIndex]
      if (!item || !state.triggerPos) return
      // Eat the trigger backtick + the auto-paired closing backtick (if
      // EasyMDE inserted one) by replacing the range from the trigger
      // through the next character with empty text first; then call
      // insertEntityRef which handles the actual `<id>` insertion plus
      // adjacency padding. We position the cursor at the trigger before
      // calling the helper so replaceSelection inserts cleanly.
      const trig = state.triggerPos
      // Probe whether the character immediately after the cursor is the
      // auto-paired closing backtick. CodeMirror inserts one when the
      // user types ` in a normal context; we want to consume it.
      const cursor = cm.getCursor()
      const nextChar = cm.getRange(cursor, { line: cursor.line, ch: cursor.ch + 1 })
      const eatRangeEnd = nextChar === '`' ? { line: cursor.line, ch: cursor.ch + 1 } : cursor
      cm.replaceRange('', trig, eatRangeEnd)
      // Cursor now sits at the trigger position; insertEntityRef writes
      // the new `<id>` code span there.
      insertEntityRef(editor as EasyMDE, item.id)
      resetSession()
      return
    }
  }

  function dismiss(): void {
    resetSession()
  }

  /* CodeMirror subscriptions ------------------------------------------- */

  cm.on('inputRead', onInputRead)
  cm.on('change', onChange)
  cm.on('cursorActivity', onCursorActivity)
  cm.on('blur', onBlur)
  cm.on('keydown', onKeyDown)

  function dispose(): void {
    cancelInflight()
    cm.off('inputRead', onInputRead)
    cm.off('change', onChange)
    cm.off('cursorActivity', onCursorActivity)
    cm.off('blur', onBlur)
    cm.off('keydown', onKeyDown)
  }

  return {
    state: readonly(state) as ReadonlySessionState,
    pick,
    moveHighlight,
    setHighlight,
    dismiss,
    dispose,
  }
}
