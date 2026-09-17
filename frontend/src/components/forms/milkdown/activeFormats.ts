/**
 * Working out which formatting is active at the cursor.
 *
 * The toolbar highlights a button when the cursor already sits in that
 * formatting, so pressing it visibly turns the formatting off rather than
 * appearing to do nothing. Without this the toolbar is a set of fire-and-hope
 * buttons that cannot tell you what state you are in.
 *
 * Kept apart from the component because it is pure state inspection: it takes
 * a ProseMirror `EditorState` and returns a set of command ids, with no DOM
 * and no editor instance involved, which is also what makes it testable.
 */
import type { EditorState } from '@milkdown/kit/prose/state'
import type { ActiveProbe, EditorCommand } from './editorCommands'

/**
 * Whether a mark is active on the current selection.
 *
 * An empty selection reads `storedMarks` first: after pressing bold with no
 * selection, the mark is pending on the next keystroke and lives there rather
 * than in the document, and a toolbar that ignored it would unlight itself the
 * moment the user armed it.
 */
function isMarkActive(state: EditorState, markName: string): boolean {
  const type = state.schema.marks[markName]
  if (!type) return false

  const { from, $from, to, empty } = state.selection
  if (empty) {
    const stored = state.storedMarks ?? $from.marks()
    return stored.some((m) => m.type === type)
  }
  return state.doc.rangeHasMark(from, to, type)
}

/**
 * Whether the selection sits inside a node of the given type.
 *
 * Walks up from the selection head so a cursor inside a list item's paragraph
 * still reports the enclosing `bullet_list`. When `match` is given the node's
 * attributes must satisfy it, which is how heading levels are told apart (an
 * equality test, built by the caller) and how a task item is recognised
 * whatever its checked state (a predicate).
 */
function isNodeActive(
  state: EditorState,
  nodeName: string,
  match?: (attrs: Record<string, unknown>) => boolean
): boolean {
  const type = state.schema.nodes[nodeName]
  if (!type) return false

  const { $from } = state.selection
  for (let depth = $from.depth; depth >= 0; depth--) {
    const node = $from.node(depth)
    if (node.type !== type) continue
    if (!match) return true
    if (match(node.attrs)) return true
  }
  return false
}

function probeMatches(state: EditorState, probe: ActiveProbe): boolean {
  switch (probe.kind) {
    case 'mark':
      return isMarkActive(state, probe.mark)
    case 'node':
      return isNodeActive(
        state,
        probe.node,
        probe.attrs && ((attrs) => Object.entries(probe.attrs!).every(([k, v]) => attrs[k] === v))
      )
    case 'nodeWhere':
      return isNodeActive(state, probe.node, probe.match)
    case 'none':
      return false
    default:
      return false
  }
}

/**
 * The ids of every command whose formatting is active at the cursor.
 *
 * Returned as a Set so the toolbar's per-button lookup stays O(1) rather than
 * rescanning the command list once per button on every selection change.
 */
export function activeCommandIds(
  state: EditorState,
  commands: readonly EditorCommand[]
): Set<string> {
  const active = new Set<string>()
  for (const cmd of commands) {
    if (probeMatches(state, cmd.probe)) active.add(cmd.id)
  }
  return active
}
