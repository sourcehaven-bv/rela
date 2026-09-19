/**
 * Placing an `entityRef` node into the document.
 *
 * Both routes to a reference end here — the `@` completion menu and the
 * toolbar's picker — so the node's shape and the cursor handling after it are
 * written once. Extracted from the editor component, which is over the
 * god-component line and was carrying two near-identical copies of this.
 *
 * A title passed in is DISPLAY ONLY and never reaches the markdown; the node
 * serializes to its id alone. It is carried so a just-inserted reference reads
 * correctly straight away, rather than showing a bare id until the next
 * mentions refresh — which would not contain the new id anyway.
 */
import { TextSelection } from '@milkdown/kit/prose/state'
import type { EditorView } from '@milkdown/kit/prose/view'
import { isValidEntityRefId } from './entityRefNode'

export interface EntityRefInsert {
  id: string
  /** Display title, or empty to let it resolve later. */
  title: string
  entityType: string | null
}

/**
 * Replaces [from, to) with an entity reference.
 *
 * Returns false when the id is not a valid reference, so a caller cannot write
 * a node the serializer would then have to make sense of.
 */
export function replaceWithEntityRef(
  view: EditorView,
  ref: EntityRefInsert,
  from: number,
  to: number
): boolean {
  if (!isValidEntityRefId(ref.id)) return false

  const { state } = view
  const nodeType = state.schema.nodes.entityRef
  if (!nodeType) return false

  const node = nodeType.create({
    id: ref.id,
    title: ref.title || null,
    entityType: ref.entityType,
    inaccessible: false,
  })

  const tr = state.tr.replaceWith(from, to, node)
  // A space after the reference, so typing on does not get absorbed into the
  // node. The node is one position wide, hence from + 1 and from + 2.
  tr.insertText(' ', from + 1)
  tr.setSelection(TextSelection.create(tr.doc, from + 2))
  view.dispatch(tr)
  return true
}

/** Inserts at the cursor, replacing any selection. */
export function insertEntityRefAtCursor(view: EditorView, ref: EntityRefInsert): boolean {
  const { from, to } = view.state.selection
  return replaceWithEntityRef(view, ref, from, to)
}

/**
 * Replaces the `@query` span before the cursor.
 *
 * `matchLength` covers the trigger and the query together, so the `@abc` the
 * user typed does not survive alongside the node it produced. A non-positive
 * length means there is no live query, and replacing would eat whatever
 * happened to precede the cursor — refused rather than guessed.
 */
export function replaceMentionQueryWithRef(
  view: EditorView,
  ref: EntityRefInsert,
  matchLength: number
): boolean {
  if (matchLength <= 0) return false
  const { $from } = view.state.selection
  const to = $from.pos
  const from = Math.max($from.start(), to - matchLength)
  return replaceWithEntityRef(view, ref, from, to)
}
