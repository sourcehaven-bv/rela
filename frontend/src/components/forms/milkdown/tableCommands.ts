/**
 * Table editing: add and remove rows and columns.
 *
 * `InsertTable` alone leaves a user with a fixed 3x3 grid and no way to change
 * it, so these are the commands that make an inserted table usable.
 *
 * Add operations come from the GFM preset (`AddRowBefore` and friends), which
 * is why they are named by slice like every other command. Deletes do NOT:
 * the preset's only delete is `DeleteSelectedCells`, which requires a
 * `CellSelection` and so reports inapplicable for an ordinary cursor — the
 * state a user is in when they click a toolbar button. These reach for
 * ProseMirror's own `deleteRow` / `deleteColumn` / `deleteTable`, which work
 * from a cursor, and are dispatched by the editor rather than through
 * `callCommand`.
 */
import type { EditorState, Transaction } from '@milkdown/kit/prose/state'
import { deleteRow, deleteColumn, deleteTable } from '@milkdown/kit/prose/tables'
import type { EditorCommand } from './editorCommands'

/** Ids of the commands this module owns, dispatched directly rather than by slice. */
export const DIRECT_TABLE_COMMANDS = new Set(['deleteRow', 'deleteColumn', 'deleteTable'])

/**
 * Ids whose availability this module decides, even though the GFM slice runs
 * them.
 *
 * `AddRowBefore` reports applicable in the header row and then corrupts the
 * table (see `canAddRowBefore`), so its button cannot be driven by the generic
 * dry run alone.
 */
export const GUARDED_TABLE_COMMANDS = new Set(['addRowBefore'])

/**
 * Whether the cursor sits in a table's header row.
 *
 * A GFM table cannot exist without its header, and `deleteRow` does not know
 * that: asked to remove the header it reports success and empties the cells
 * instead, leaving a table whose first row is a pair of `<br />`. Callers use
 * this to refuse rather than produce that.
 */
export function isInHeaderRow(state: EditorState): boolean {
  const { $from } = state.selection
  for (let depth = $from.depth; depth > 0; depth--) {
    if ($from.node(depth).type.name === 'table_header_row') return true
  }
  return false
}

/** Whether the cursor is anywhere inside a table. */
export function isInTable(state: EditorState): boolean {
  const { $from } = state.selection
  for (let depth = $from.depth; depth > 0; depth--) {
    if ($from.node(depth).type.name === 'table') return true
  }
  return false
}

/**
 * Whether the cursor is in the only body row of its table.
 *
 * The GFM table schema is `table_header_row table_row+`, so the last body row
 * cannot be removed. `deleteRow` does not refuse: it reports success and
 * blanks the cells, leaving a row of empty strings where the user expected
 * none. Deleting the whole table is the operation they actually want, and it
 * has its own button.
 */
export function isOnlyBodyRow(state: EditorState): boolean {
  const { $from } = state.selection
  for (let depth = $from.depth; depth > 0; depth--) {
    if ($from.node(depth).type.name !== 'table_row') continue
    const table = $from.node(depth - 1)
    if (table.type.name !== 'table') return false
    let bodyRows = 0
    table.forEach((child) => {
      if (child.type.name === 'table_row') bodyRows += 1
    })
    return bodyRows <= 1
  }
  return false
}

/**
 * Whether a row may be inserted above the cursor.
 *
 * There is no position above the header row: a GFM table opens with it. Asked
 * anyway, `AddRowBefore` inserts a body row at index 0 and leaves the header
 * EMPTY, which serializes as a headerless table immediately followed by a
 * second one — the original, now orphaned. Refusing is the only correct
 * answer, and "insert row below" already covers what the user meant.
 */
export function canAddRowBefore(state: EditorState): boolean {
  return isInTable(state) && !isInHeaderRow(state)
}

/**
 * Whether a directly-dispatched table command would apply.
 *
 * Mirrors what `runTableCommand` will do, so the toolbar's disabled state and
 * the command's behaviour cannot disagree.
 */
export function canRunTableCommand(id: string, state: EditorState): boolean {
  switch (id) {
    case 'deleteRow':
      // Refuses in the header and on the last body row, neither of which can
      // actually be removed; see `isInHeaderRow` and `isOnlyBodyRow`.
      return !isInHeaderRow(state) && !isOnlyBodyRow(state) && deleteRow(state)
    case 'deleteColumn':
      return deleteColumn(state)
    case 'deleteTable':
      return deleteTable(state)
    default:
      return false
  }
}

/**
 * Runs a directly-dispatched table command.
 *
 * Returns false when the command does not apply, so the caller can leave the
 * document untouched.
 */
export function runTableCommand(
  id: string,
  state: EditorState,
  dispatch: (tr: Transaction) => void
): boolean {
  if (!canRunTableCommand(id, state)) return false
  switch (id) {
    case 'deleteRow':
      return deleteRow(state, dispatch)
    case 'deleteColumn':
      return deleteColumn(state, dispatch)
    case 'deleteTable':
      return deleteTable(state, dispatch)
    default:
      return false
  }
}

/**
 * The table commands, offered in their own toolbar group.
 *
 * They stay out of `BLOCK_COMMANDS` because they are not block types a user
 * turns the current paragraph into — they act on a table that already exists,
 * and the whole group hides when the cursor is not in one.
 */
export const TABLE_COMMANDS: EditorCommand[] = [
  {
    id: 'addRowBefore',
    label: 'Insert row above',
    command: 'AddRowBefore',
    probe: { kind: 'none' },
    keywords: ['row', 'above', 'before'],
  },
  {
    id: 'addRowAfter',
    label: 'Insert row below',
    command: 'AddRowAfter',
    probe: { kind: 'none' },
    keywords: ['row', 'below', 'after'],
  },
  {
    id: 'addColBefore',
    label: 'Insert column left',
    command: 'AddColBefore',
    probe: { kind: 'none' },
    keywords: ['column', 'left', 'before'],
  },
  {
    id: 'addColAfter',
    label: 'Insert column right',
    command: 'AddColAfter',
    probe: { kind: 'none' },
    keywords: ['column', 'right', 'after'],
  },
  {
    id: 'deleteRow',
    label: 'Delete row',
    command: 'DeleteRow',
    probe: { kind: 'none' },
    keywords: ['delete', 'remove', 'row'],
  },
  {
    id: 'deleteColumn',
    label: 'Delete column',
    command: 'DeleteColumn',
    probe: { kind: 'none' },
    keywords: ['delete', 'remove', 'column'],
  },
  {
    id: 'deleteTable',
    label: 'Delete table',
    command: 'DeleteTable',
    probe: { kind: 'none' },
    keywords: ['delete', 'remove', 'table'],
  },
]
