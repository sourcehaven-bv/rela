// Insertion helper for the markdown editor's entity-reference picker
// (TKT-I5NO). The picker emits a selected entity ID; this helper wraps it
// in backticks and inserts it at the cursor (or replaces the current
// selection) via CodeMirror's `replaceSelection`. Pulled out of the
// MarkdownEditor component so it can be unit-tested without standing up
// EasyMDE/CodeMirror in JSDOM.
import type EasyMDE from 'easymde'

// Reject IDs that would break the inserted code span or the underlying
// store's bucket-key invariants. Mirrors the backend's
// internal/store/storeutil.ValidateID denylist (RR-D54M) plus the
// code-span-specific rejection of backticks; the rest of the universe
// (leading digits, dots, hyphens, manual IDs like "iso-27001-a.5.1") is
// allowed because the store accepts those too. A 1024-byte cap guards
// against pathological inputs without inventing a tighter grammar than
// the backend.
const MAX_ID_BYTES = 1024
const FORBIDDEN_RUN = '--'

function isValidId(id: string): boolean {
  if (typeof id !== 'string' || id === '' || id.length > MAX_ID_BYTES) return false
  if (id.includes(FORBIDDEN_RUN)) return false
  for (let i = 0; i < id.length; i++) {
    const code = id.charCodeAt(i)
    // Reject ASCII control characters (including NUL, newline, CR, tab).
    if (code < 0x20 || code === 0x7f) return false
    // Path separators that storeutil rejects.
    if (code === 0x2f /* / */ || code === 0x5c /* \ */) return false
    // Backticks would close the inserted code span early; spaces would
    // break exact-match insertion. Neither is in storeutil's denylist
    // because the store wouldn't accept them in IDs anyway, but we
    // belt-and-brace.
    if (code === 0x60 /* ` */ || code === 0x20 /* space */) return false
  }
  return true
}

// Minimal subset of the CodeMirror v5 surface we use. Typed as a
// structural interface so tests can pass a plain object that records the
// call instead of constructing a real CodeMirror. `getCursor(side)`
// matches CM5: 'from' is the lower bound of the primary selection,
// 'to' is the upper bound — independent of which end is the head.
interface CodeMirrorLike {
  getCursor: (side?: 'from' | 'to' | 'anchor' | 'head') => { line: number; ch: number }
  getRange: (from: { line: number; ch: number }, to: { line: number; ch: number }) => string
  replaceSelection: (text: string, select?: 'around' | 'start' | 'end') => void
  focus: () => void
}

// Either an EasyMDE instance (production) or a minimal duck-typed shim
// (tests). Both expose `.codemirror` with the methods above.
type EditorLike = Pick<EasyMDE, 'codemirror'> | { codemirror: CodeMirrorLike } | null | undefined

/**
 * Insert `\`<id>\`` at the editor's cursor, replacing any active selection.
 *
 * Adjacency-aware: if the character immediately to the left or right of
 * the selection bounds is a backtick, the inserted text is padded with a
 * single space on that side so the new code span parses as its own
 * inline token rather than concatenating with the neighboring backticks
 * (RR-NKV5). Adjacency is computed against the SELECTION bounds (`from`
 * / `to`), not the cursor head — so a non-empty selection adjacent to a
 * code span is handled correctly regardless of selection direction
 * (RR-A4RR).
 *
 * Silent no-op when:
 *   - `editor` is null/undefined (the parent's MarkdownEditor may have
 *     been torn down while the picker was open — RR-032O);
 *   - `id` is invalid per `isValidId` (RR-D54M).
 */
export function insertEntityRef(editor: EditorLike, id: string): void {
  if (!editor || !editor.codemirror) return
  if (!isValidId(id)) return

  const cm = editor.codemirror as CodeMirrorLike
  // `getCursor('from')` and `getCursor('to')` return the selection
  // bounds regardless of selection direction (forward vs. backward).
  // For an empty selection both return the cursor position.
  const from = cm.getCursor('from')
  const to = cm.getCursor('to')
  const leftAdjacent = readAdjacent(cm, from, -1)
  const rightAdjacent = readAdjacent(cm, to, 1)

  const leftPad = leftAdjacent === '`' ? ' ' : ''
  const rightPad = rightAdjacent === '`' ? ' ' : ''
  const text = `${leftPad}\`${id}\`${rightPad}`

  cm.replaceSelection(text, 'end')
}

// readAdjacent returns the character one position away from `pos` in the
// given direction (-1 for left, +1 for right), or '' when the position is
// at a buffer edge. Uses getRange rather than getLine so we don't need to
// track end-of-line semantics — getRange on a one-char span just returns
// the char (or '' at the boundary).
function readAdjacent(
  cm: CodeMirrorLike,
  pos: { line: number; ch: number },
  direction: -1 | 1,
): string {
  if (direction === -1) {
    if (pos.ch === 0) return ''
    return cm.getRange({ line: pos.line, ch: pos.ch - 1 }, pos)
  }
  return cm.getRange(pos, { line: pos.line, ch: pos.ch + 1 })
}
