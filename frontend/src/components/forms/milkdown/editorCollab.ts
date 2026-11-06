/**
 * The parts of MilkdownEditor's co-editing mode that do not need the
 * component (TKT-CJL604).
 */
import type { Ctx } from '@milkdown/kit/ctx'
import { parserCtx } from '@milkdown/kit/core'
import type { EditorState, Transaction } from '@milkdown/kit/prose/state'
import type { EditorView } from '@milkdown/kit/prose/view'
import { collabServiceCtx } from '@milkdown/plugin-collab'
import {
  prosemirrorToYXmlFragment,
  undo,
  redo,
  yUndoPluginKey,
  ySyncPluginKey,
} from 'y-prosemirror'
import type { EditorCollab } from '@/composables/collab/collabRoom'

/**
 * Seeds an empty room from this client's body, then binds the view to it.
 *
 * Call only once the room has synced: only then does the client know whether
 * the room already holds a document. Seeding is deterministic, so two clients
 * that both find the room empty write the same items and merge into one copy.
 */
export function bindToRoom(ctx: Ctx, c: EditorCollab): void {
  const parse = ctx.get(parserCtx)
  c.room.seed(c.base, (fragment) => prosemirrorToYXmlFragment(parse(c.base.content), fragment))
  ctx
    .get(collabServiceCtx)
    .mergeOptions({ yCursorOpts: { cursorBuilder: peerCursor, selectionBuilder: peerSelection } })
    .bindDoc(c.room.doc)
    .setAwareness(c.room.awareness)
    .connect()
}

/**
 * How many steps undo and redo can take in a room.
 *
 * y-prosemirror's undo manager tracks this client's own changes only;
 * ProseMirror's history would undo a peer's typing.
 */
export function roomHistoryDepths(state: EditorState): { undo: number; redo: number } {
  const manager = yUndoPluginKey.getState(state)?.undoManager
  return { undo: manager?.undoStack.length ?? 0, redo: manager?.redoStack.length ?? 0 }
}

const HISTORY_EVENTS = ['stack-item-added', 'stack-item-popped', 'stack-cleared'] as const

/**
 * Calls `changed` whenever the room's undo or redo stack changes, and returns
 * the unsubscribe. The stack grows after the ProseMirror transaction that
 * caused it, so a toolbar refreshed from that transaction reads the old depth.
 */
export function onRoomHistoryChange(state: EditorState, changed: () => void): () => void {
  const manager = yUndoPluginKey.getState(state)?.undoManager
  if (!manager) return () => {}
  for (const name of HISTORY_EVENTS) manager.on(name, changed)
  return () => {
    for (const name of HISTORY_EVENTS) manager.off(name, changed)
  }
}

/** Runs undo or redo through the room's undo manager. */
export function runRoomHistory(id: 'undo' | 'redo', view: EditorView): void {
  ;(id === 'undo' ? undo : redo)(view.state)
}

/** The longest peer name a caret shows. */
export const PEER_NAME_MAX = 64

const PEER_COLOR = /^#[0-9a-f]{6}$/i

/**
 * A peer's caret. The color goes through a custom property set from script
 * rather than a `style` attribute, which a `style-src` without
 * `'unsafe-inline'` would block.
 *
 * A peer writes its own awareness state, so both fields are untrusted: the
 * color is used only when it is a `#rrggbb` value, and the name is cut to
 * `PEER_NAME_MAX` characters.
 */
export function peerCursor(user: { name?: unknown; color?: unknown }): HTMLElement {
  const caret = document.createElement('span')
  caret.classList.add('ProseMirror-yjs-cursor')
  if (typeof user.color === 'string' && PEER_COLOR.test(user.color)) {
    caret.style.setProperty('--collab-color', user.color)
  }
  const label = document.createElement('div')
  label.textContent = typeof user.name === 'string' ? user.name.slice(0, PEER_NAME_MAX) : ''
  // Word joiners keep the caret from splitting the word it sits in.
  caret.append(document.createTextNode('⁠'), label, document.createTextNode('⁠'))
  return caret
}

const selectionClass = 'ProseMirror-yjs-selection'

/** A peer's selection. A class only, for the same reason as the caret. */
export function peerSelection(): { class: string } {
  return { class: selectionClass }
}

/**
 * Whether `tr` applies a change from the room's document: a peer's edit, or
 * this client's own undo or redo, which also runs through the document.
 * y-prosemirror marks these `addToHistory: false`, and Milkdown's markdown
 * listener skips such transactions, so the editor must emit for them itself.
 */
export function isRoomChange(tr: Transaction): boolean {
  if (!tr.docChanged) return false
  const meta = tr.getMeta(ySyncPluginKey) as { isChangeOrigin?: boolean } | undefined
  return meta?.isChangeOrigin === true
}
