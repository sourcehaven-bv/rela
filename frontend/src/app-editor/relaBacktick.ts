// Inline backtick-triggered entity-reference autocomplete for <rela-editor>
// (TKT-D2JML7). A plain-DOM, framework-free, bridge-fed port of the SPA's
// useBacktickAutocomplete (TKT-2RCP): type `` ` `` → pick a type prefix
// (TKT-, FEAT-, …) → pick an entity → inserts `` `<ID>` `` at the cursor.
//
// Differences from the SPA version (by necessity / scope):
//   - Data comes through the bridge (window.rela.schema/search/list), NOT axios
//     — the app iframe has `connect-src 'none'` so it cannot fetch directly.
//   - The popup is plain DOM (no Vue), themed via _rela.css tokens.
//   - Insertion reuses the same `` `<id>` `` + adjacency-padding rule as
//     insertEntityRef.ts (kept in sync; see that file for the rationale).
//
// The state machine mirrors the SPA: idle → pending(open-delay) → prefix → id.

import type EasyMDE from 'easymde'

const OPEN_DELAY_MS = 250
const SEARCH_DEBOUNCE_MS = 150
const MAX_RESULTS = 50
const MIN_SEARCH_LEN = 1
// Typing any of these ends the trigger window (can't be part of an entity ID).
const NON_ID_CHAR = /[\s`(){}[\]<>,;:!?"'\\]/

// --- bridge surface this module needs (the app injects window.rela) ---
interface Bridge {
  schema(): Promise<SchemaResponse>
  search(p: { query: string; type?: string }): Promise<{ data?: EntityRow[] }>
  list(p: { type: string; params?: Record<string, unknown> }): Promise<{ data?: EntityRow[] }>
}
interface SchemaResponse {
  entities: Record<string, SchemaEntity>
}
interface SchemaEntity {
  label?: string
  id_prefix?: string
  id_type?: string
}
interface EntityRow {
  id: string
  // _title is the server-computed display title (honours the type's
  // display_property), so we use it directly rather than reading
  // properties.title (which would show bare IDs for non-title display props —
  // BUG-1P88YM). The SPA's entityDisplayTitle() util isn't importable into this
  // standalone IIFE, but _title already encodes the same decision server-side.
  _title?: string
}

interface PrefixItem {
  prefix: string // "" for manual types
  type: string
  label: string
  isManual: boolean
}

type Phase = 'idle' | 'pending' | 'prefix' | 'id'

// Minimal CodeMirror 5 surface used here (EasyMDE's editor.codemirror).
interface CM {
  on(ev: string, fn: (...a: unknown[]) => void): void
  off(ev: string, fn: (...a: unknown[]) => void): void
  getCursor(side?: string): { line: number; ch: number }
  getRange(a: { line: number; ch: number }, b: { line: number; ch: number }): string
  getTokenAt(pos: { line: number; ch: number }, precise?: boolean): { type: string | null; string: string }
  charCoords(pos: { line: number; ch: number }, mode: string): { left: number; top: number; bottom: number }
  replaceSelection(text: string, select?: string): void
  replaceRange(text: string, from: { line: number; ch: number }, to?: { line: number; ch: number }): void
  focus(): void
}

// --- id validation: mirror insertEntityRef.ts's isValidId ---
const MAX_ID_BYTES = 1024
function isValidId(id: string): boolean {
  if (typeof id !== 'string' || id === '' || id.length > MAX_ID_BYTES) return false
  if (id.includes('--')) return false
  for (let i = 0; i < id.length; i++) {
    const c = id.charCodeAt(i)
    if (c < 0x20 || c === 0x7f) return false
    if (c === 0x2f || c === 0x5c || c === 0x60 || c === 0x20) return false
  }
  return true
}

// Insert `` `<id>` `` at the cursor with adjacency padding (mirror of
// insertEntityRef). The trigger backtick + typed text are first removed.
function insertRef(cm: CM, id: string): void {
  if (!isValidId(id)) return
  const from = cm.getCursor('from')
  const to = cm.getCursor('to')
  const left = from.ch === 0 ? '' : cm.getRange({ line: from.line, ch: from.ch - 1 }, from)
  const right = cm.getRange(to, { line: to.line, ch: to.ch + 1 })
  const leftPad = left === '`' ? ' ' : ''
  const rightPad = right === '`' ? ' ' : ''
  cm.replaceSelection(`${leftPad}\`${id}\`${rightPad}`, 'end')
}

// --- token-context rules (mirror of the SPA's two-side classification) ---
// hasFormatting: the token AFTER the trigger must itself be a CM "formatting"
// token (the backtick that opens a code span), so we only open when the `` ` ``
// actually starts inline code — not inside a fenced block, URL, etc. The token
// type is a space-joined class list like "formatting formatting-code comment".
function hasFormatting(tok: { type: string | null } | null): boolean {
  return !!(tok && tok.type && tok.type.indexOf('formatting') !== -1)
}
// isInlineTextContext: the token BEFORE the trigger must not be one of these
// non-inline-text contexts (we'd be inside a comment/code/link/etc.).
const SUPPRESS_TYPES = ['comment', 'code', 'link', 'url', 'string', 'meta', 'tag']
function isInlineTextContext(tok: { type: string | null } | null): boolean {
  if (!tok || !tok.type) return true
  for (const bad of SUPPRESS_TYPES) {
    if (tok.type.indexOf(bad) !== -1) return false
  }
  return true
}

function entityTitle(e: EntityRow): string {
  return e._title || e.id
}

export interface BacktickController {
  destroy(): void
}

export function attachBacktickAutocomplete(editor: EasyMDE, bridge: Bridge): BacktickController {
  const cm = editor.codemirror as unknown as CM

  let phase: Phase = 'idle'
  let triggerPos: { line: number; ch: number } | null = null
  let prefixItems: PrefixItem[] = []
  let entityItems: EntityRow[] = []
  let highlighted = 0
  let resolvedPrefix: PrefixItem | null = null
  let allPrefixes: PrefixItem[] = []
  let openTimer: ReturnType<typeof setTimeout> | null = null
  let searchTimer: ReturnType<typeof setTimeout> | null = null
  let searchSeq = 0
  let cachedPrefixes: PrefixItem[] | null = null

  // --- popup DOM ---
  const popup = document.createElement('div')
  popup.className = 'rela-bt-popup'
  popup.setAttribute('role', 'listbox')
  popup.style.display = 'none'
  // mousedown must not steal focus from the editor's textarea
  popup.addEventListener('mousedown', (e) => e.preventDefault())
  document.body.appendChild(popup)

  function buildPrefixList(schema: SchemaResponse): PrefixItem[] {
    const items: PrefixItem[] = []
    for (const [type, def] of Object.entries(schema.entities || {})) {
      const label = def.label || type
      const prefix = def.id_prefix || ''
      const isManual = def.id_type === 'manual' || prefix === ''
      items.push({ prefix, type, label, isManual })
    }
    items.sort((a, b) => a.label.localeCompare(b.label))
    return items
  }

  async function ensurePrefixes(): Promise<PrefixItem[]> {
    if (cachedPrefixes) return cachedPrefixes
    try {
      const schema = await bridge.schema()
      cachedPrefixes = buildPrefixList(schema)
    } catch {
      cachedPrefixes = []
    }
    return cachedPrefixes
  }

  function reset(): void {
    if (openTimer) { clearTimeout(openTimer); openTimer = null }
    if (searchTimer) { clearTimeout(searchTimer); searchTimer = null }
    phase = 'idle'
    triggerPos = null
    prefixItems = []
    entityItems = []
    highlighted = 0
    resolvedPrefix = null
    popup.style.display = 'none'
  }

  function typedAfterTrigger(): string {
    if (!triggerPos) return ''
    const cur = cm.getCursor()
    if (cur.line !== triggerPos.line || cur.ch <= triggerPos.ch) return ''
    return cm.getRange({ line: triggerPos.line, ch: triggerPos.ch + 1 }, cur)
  }

  function position(): void {
    if (!triggerPos) return
    const c = cm.charCoords({ line: triggerPos.line, ch: triggerPos.ch }, 'window')
    popup.style.left = `${Math.round(c.left)}px`
    popup.style.top = `${Math.round(c.bottom + 2)}px`
  }

  function render(): void {
    const rows: Array<{ label: string; sub: string }> =
      phase === 'prefix'
        ? prefixItems.map((p) => ({ label: p.label, sub: p.isManual ? '(manual)' : `${p.prefix}*` }))
        : entityItems.map((e) => ({ label: entityTitle(e), sub: e.id }))

    if (rows.length === 0) {
      popup.style.display = 'none'
      return
    }
    if (highlighted >= rows.length) highlighted = rows.length - 1
    if (highlighted < 0) highlighted = 0

    popup.replaceChildren()
    rows.forEach((r, idx) => {
      const li = document.createElement('div')
      li.className = 'rela-bt-option' + (idx === highlighted ? ' active' : '')
      li.setAttribute('role', 'option')
      const title = document.createElement('span')
      title.className = 'rela-bt-title'
      title.textContent = r.label
      const sub = document.createElement('span')
      sub.className = 'rela-bt-sub'
      sub.textContent = r.sub
      li.append(title, sub)
      li.addEventListener('mouseenter', () => { highlighted = idx; render() })
      li.addEventListener('click', () => choose(idx))
      popup.appendChild(li)
    })
    position()
    popup.style.display = 'block'
  }

  function filterPrefix(typed: string): void {
    const upper = typed.toUpperCase()
    const lower = typed.toLowerCase()
    prefixItems = allPrefixes.filter((p) =>
      p.isManual
        ? p.label.toLowerCase().startsWith(lower)
        : p.prefix.toUpperCase().startsWith(upper) || upper.startsWith(p.prefix.toUpperCase()),
    )
    if (prefixItems.length === 0) { reset(); return }
  }

  function exactPrefix(typed: string): PrefixItem | null {
    const upper = typed.toUpperCase()
    let best: PrefixItem | null = null
    for (const p of prefixItems) {
      if (p.isManual || p.prefix.length === 0) continue
      const pu = p.prefix.toUpperCase()
      const startsWith = upper.startsWith(pu) && upper.length >= pu.length
      const startsWithDash = !pu.endsWith('-') && upper.startsWith(pu + '-')
      if (!startsWith && !startsWithDash) continue
      if (best === null || p.prefix.length > best.prefix.length) best = p
    }
    return best
  }

  function toIdPhase(prefix: PrefixItem): void {
    phase = 'id'
    resolvedPrefix = prefix
    entityItems = []
    highlighted = 0
    scheduleSearch('')
  }

  function scheduleSearch(query: string): void {
    if (searchTimer) clearTimeout(searchTimer)
    searchTimer = setTimeout(() => void runSearch(query), SEARCH_DEBOUNCE_MS)
  }

  async function runSearch(query: string): Promise<void> {
    if (!resolvedPrefix) return
    const type = resolvedPrefix.type
    const seq = ++searchSeq
    try {
      const resp =
        query.length >= MIN_SEARCH_LEN
          ? await bridge.search({ query, type })
          : await bridge.list({ type, params: { per_page: MAX_RESULTS } })
      if (seq !== searchSeq || phase !== 'id') return // superseded / closed
      entityItems = (resp.data || []).slice(0, MAX_RESULTS)
      highlighted = 0
      render()
    } catch {
      if (seq !== searchSeq) return
      entityItems = []
      render()
    }
  }

  function applyTyped(typed: string): void {
    if (phase === 'id' && resolvedPrefix && resolvedPrefix.prefix) {
      if (!typed.toUpperCase().startsWith(resolvedPrefix.prefix.toUpperCase())) {
        // backspaced out of the id body → back to prefix phase
        phase = 'prefix'
        resolvedPrefix = null
        prefixItems = allPrefixes
      }
    }
    if (phase === 'prefix') {
      filterPrefix(typed)
      if (phase !== 'prefix') return
      const exact = exactPrefix(typed)
      if (exact) { toIdPhase(exact); return }
      render()
    } else if (phase === 'id') {
      const prefix = resolvedPrefix?.prefix ?? ''
      let partial = typed.toUpperCase().startsWith(prefix.toUpperCase()) ? typed.slice(prefix.length) : typed
      if (partial.startsWith('-')) partial = partial.slice(1)
      scheduleSearch(partial)
    }
  }

  function choose(idx: number): void {
    const trig = triggerPos
    if (!trig) return
    if (phase === 'prefix') {
      const item = prefixItems[idx]
      if (!item) return
      // Replace the typed-so-far with the prefix text, then go to id phase.
      const cur = cm.getCursor()
      cm.replaceRange(item.prefix, { line: trig.line, ch: trig.ch + 1 }, cur)
      phase = 'prefix' // applyTyped will move to id
      filterPrefix(item.prefix)
      const exact = exactPrefix(item.prefix)
      if (exact) toIdPhase(exact)
      else render()
      cm.focus()
      return
    }
    if (phase === 'id') {
      const ent = entityItems[idx]
      if (!ent) return
      // Remove the trigger backtick + everything typed after it, then insert
      // the proper `<id>` code span.
      const cur = cm.getCursor()
      cm.replaceRange('', { line: trig.line, ch: trig.ch }, cur)
      insertRef(cm, ent.id)
      reset()
      cm.focus()
    }
  }

  // --- CM event handlers ---
  function onInputRead(_cm: unknown, change: { text: string[]; from: { line: number; ch: number } }): void {
    if (phase !== 'idle') return
    if (change.text.length !== 1 || change.text[0] !== '`') return
    const line = change.from.line
    const ch = change.from.ch
    const after = cm.getTokenAt({ line, ch: ch + 1 }, true)
    const before = ch > 0 ? cm.getTokenAt({ line, ch: ch - 1 }, true) : null
    if (!hasFormatting(after)) return
    if (before && !isInlineTextContext(before)) return

    triggerPos = { line, ch }
    phase = 'pending'
    openTimer = setTimeout(() => {
      openTimer = null
      if (phase !== 'pending') return
      const typed = typedAfterTrigger()
      if (NON_ID_CHAR.test(typed)) { reset(); return }
      phase = 'prefix'
      allPrefixes = cachedPrefixes || []
      prefixItems = allPrefixes
      highlighted = 0
      render()
      if (typed.length > 0) applyTyped(typed)
    }, OPEN_DELAY_MS)
  }

  function onChange(): void {
    if (phase === 'idle') return
    const cur = cm.getCursor()
    if (!triggerPos || cur.line !== triggerPos.line || cur.ch <= triggerPos.ch) { reset(); return }
    const typed = typedAfterTrigger()
    if (NON_ID_CHAR.test(typed)) { reset(); return }
    if (phase === 'pending') return
    applyTyped(typed)
  }

  // cursorActivity is SEPARATE from change and must be tolerant: arrow-key
  // navigation in the popup must NOT close the session. Only close when the
  // caret leaves the trigger's line or retreats to/before the trigger (e.g. a
  // click elsewhere). We intentionally do NOT re-run applyTyped here (that's
  // the change handler's job) — doing so on every caret move is what closed the
  // popup on ArrowUp/Down. (TKT-D2JML7)
  function onCursorActivity(): void {
    if (phase === 'idle') return
    const cur = cm.getCursor()
    if (!triggerPos || cur.line !== triggerPos.line || cur.ch <= triggerPos.ch) reset()
  }

  function onKeyDown(_cm: unknown, ev: KeyboardEvent): void {
    if (phase !== 'prefix' && phase !== 'id') return
    const count = phase === 'prefix' ? prefixItems.length : entityItems.length
    if (count === 0) return
    switch (ev.key) {
      case 'ArrowDown':
        ev.preventDefault(); highlighted = (highlighted + 1) % count; render(); break
      case 'ArrowUp':
        ev.preventDefault(); highlighted = (highlighted - 1 + count) % count; render(); break
      case 'Enter':
      case 'Tab':
        ev.preventDefault(); choose(highlighted); break
      case 'Escape':
        ev.preventDefault(); reset(); break
    }
  }

  // Warm the prefix cache so the first trigger is instant.
  void ensurePrefixes().then((p) => { cachedPrefixes = p })

  const onBlur = (): void => { setTimeout(reset, 150) } // let a popup click land first

  cm.on('inputRead', onInputRead as (...a: unknown[]) => void)
  cm.on('change', onChange)
  cm.on('keydown', onKeyDown as (...a: unknown[]) => void)
  cm.on('cursorActivity', onCursorActivity)
  cm.on('blur', onBlur)

  return {
    destroy() {
      reset()
      cm.off('inputRead', onInputRead as (...a: unknown[]) => void)
      cm.off('change', onChange)
      cm.off('keydown', onKeyDown as (...a: unknown[]) => void)
      cm.off('cursorActivity', onCursorActivity)
      cm.off('blur', onBlur)
      if (popup.parentNode) popup.parentNode.removeChild(popup)
    },
  }
}
