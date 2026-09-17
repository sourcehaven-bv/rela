/**
 * Reading the link at (or around) the selection.
 *
 * Pure state inspection, like `activeFormats.ts`: it takes a ProseMirror
 * `EditorState` and returns plain data, so the decisions it drives can be
 * tested without mounting an editor.
 *
 * This exists because "insert a link" and "change this link" are the same
 * button, and telling them apart is not optional. `ToggleLink` is ProseMirror's
 * `toggleMark`, whose `removeWhenPresent` defaults to true and decides with
 *
 *     add = !ranges.some(r => doc.rangeHasMark(r.$from.pos, r.$to.pos, type))
 *
 * `.some`, not `.every`. So a selection that overlaps an existing link even
 * partially takes the REMOVE branch: run it over `see [docs](url) here` and the
 * existing link is destroyed and whatever URL the user just typed is discarded,
 * with nothing visibly wrong until save. `findLinkAt` is what lets the caller
 * route that case to a retarget instead.
 */
import type { EditorState } from '@milkdown/kit/prose/state'
import type { Mark } from '@milkdown/kit/prose/model'

export interface LinkAtSelection {
  /** Current target, for prefilling the dialog. */
  href: string
  /** Document range the link mark covers, for a precise retarget. */
  from: number
  to: number
  /** The link's text, so a dialog can offer to edit it. */
  text: string
}

/**
 * Finds the link mark the selection sits in or overlaps.
 *
 * Returns the FULL extent of the link, not the intersection with the
 * selection. Retargeting half a link is not a thing a user means, and
 * `UpdateLink` would not do it anyway — it rewrites the matched node's whole
 * `nodeSize`.
 *
 * Nil: returns `null` when no link is involved, which is the caller's signal
 * that this is an insert rather than an edit.
 */
export function findLinkAt(state: EditorState): LinkAtSelection | null {
  const linkType = state.schema.marks.link
  if (!linkType) return null

  const { from, to, empty, $from } = state.selection

  // An empty selection is the common case (caret resting in a link). Scanning
  // the node under the cursor is enough and avoids a doc walk.
  if (empty) {
    const mark = $from.marks().find((m) => m.type === linkType)
    if (!mark) return null
    return extentOf(state, $from.pos, mark)
  }

  // A non-empty selection: find the first position carrying a link mark
  // anywhere in range, then expand to that link's full extent.
  let foundPos = -1
  let foundMark: Mark | null = null
  state.doc.nodesBetween(from, to, (node, pos) => {
    if (foundMark) return false
    if (!node.isText) return true
    const mark = node.marks.find((m) => m.type === linkType)
    if (mark) {
      foundPos = pos
      foundMark = mark
    }
    return true
  })
  if (!foundMark) return null
  // `nodesBetween` gives the node's start; +1 puts us inside it, which is what
  // the extent walk expects for a resolvable position.
  return extentOf(state, foundPos + 1, foundMark)
}

/**
 * Expands a position carrying `mark` to the whole run of that same mark.
 *
 * "Same" means the identical mark instance (`Mark.eq`), so two adjacent links
 * with different targets stay separate rather than merging into one range.
 */
function extentOf(state: EditorState, pos: number, mark: Mark): LinkAtSelection | null {
  const $pos = state.doc.resolve(pos)
  const parent = $pos.parent
  const parentStart = $pos.start()

  let index = $pos.index()
  // A caret at the very end of a text node resolves to the following index,
  // which carries no mark; step back so the link under the cursor is found.
  if (index >= parent.childCount || !mark.isInSet(parent.child(index).marks)) {
    if (index > 0 && mark.isInSet(parent.child(index - 1).marks)) index -= 1
    else return null
  }

  let start = parentStart
  for (let i = 0; i < index; i += 1) start += parent.child(i).nodeSize

  let end = start + parent.child(index).nodeSize
  let text = parent.child(index).text ?? ''

  // Walk outward over neighbouring text nodes carrying the same mark. They are
  // separate nodes whenever another mark (bold, code) starts or stops inside
  // the link, so a link with a bold word in it is several nodes, one link.
  for (let i = index - 1; i >= 0; i -= 1) {
    const child = parent.child(i)
    if (!child.isText || !mark.isInSet(child.marks)) break
    start -= child.nodeSize
    text = (child.text ?? '') + text
  }
  for (let i = index + 1; i < parent.childCount; i += 1) {
    const child = parent.child(i)
    if (!child.isText || !mark.isInSet(child.marks)) break
    end += child.nodeSize
    text += child.text ?? ''
  }

  return { href: String(mark.attrs.href ?? ''), from: start, to: end, text }
}

/**
 * Whether a link mark can be applied over the current selection.
 *
 * Used to disable the toolbar button rather than let a press do nothing. The
 * interesting refusal is a code block, whose schema is `marks: ''` — the mark
 * genuinely cannot apply there, so this needs no special case beyond asking
 * the schema.
 */
export function canApplyLink(state: EditorState): boolean {
  const linkType = state.schema.marks.link
  if (!linkType) return false

  const { $from, $to, empty } = state.selection
  if (empty) return $from.parent.type.allowsMarkType(linkType)

  let allowed = false
  state.doc.nodesBetween($from.pos, $to.pos, (node, _pos, parent) => {
    if (allowed) return false
    if (node.isText && parent?.type.allowsMarkType(linkType)) allowed = true
    return true
  })
  return allowed
}
