/**
 * The configuration edit behind "Add column" on a kanban board (TKT-1X5G3BP).
 *
 * A column on a `column_property` board is a value of that property's enum,
 * so a new column is a schema write. It goes through the Configure API, the
 * only server-side config writer. This module only edits the trees; the
 * board sends them with `saveConfigure`.
 *
 * Only an inline enum is edited (`type: enum` with its own `values:`). A
 * named type's values are shared with every property that uses the type, and
 * the server refuses `add_column` on such a board at config load.
 */
import type { ConfigDraftBody, ConfigSnapshot } from '@/api/configure'
import {
  clone,
  ensureList,
  ensureMap,
  get,
  mapAt,
  mapItems,
  newMap,
  set,
  str,
  strList,
} from './tree'
import { machineName } from './names'

export interface AddColumnTarget {
  kanbanId: string
  entityType: string
  property: string
}

/**
 * Why the edit cannot be made. `exists` carries the existing column's name.
 * `unsupported` means the property is not an inline enum in this snapshot.
 */
type AddColumnRefusal =
  { reason: 'no_name' } | { reason: 'exists'; name: string } | { reason: 'unsupported' }

export type AddColumnEdit = { value: string; draft: ConfigDraftBody } | AddColumnRefusal

interface Option {
  value: string
  label: string
}

/** The option a typed name stands for: the same value, or the same label in any case. */
function matching(options: Option[], value: string, name: string): Option | undefined {
  const folded = name.toLowerCase()
  return options.find((o) => o.value === value || o.label.toLowerCase() === folded)
}

/**
 * The draft that adds a column labelled `label` to the board: the value
 * appended to the property's `values:`, a `labels:` entry when the label is
 * not the value itself, and an item on the board's `columns:` when it lists
 * them. The snapshot is not changed.
 *
 * A board that lists `columns:` may leave out values the property has. Naming
 * one of those shows it: the column is added to the board, and the schema is
 * left alone.
 */
export function addEnumColumn(
  snapshot: ConfigSnapshot,
  target: AddColumnTarget,
  label: string
): AddColumnEdit {
  const name = label.trim()
  const value = machineName(name)
  if (!value) return { reason: 'no_name' }

  const schema = clone(snapshot.schema)
  const prop = mapAt(schema, ['entities', target.entityType, 'properties', target.property])
  if (!prop || str(get(prop, 'type')) !== 'enum') return { reason: 'unsupported' }

  const labels = get(prop, 'labels')
  const values: Option[] = strList(get(prop, 'values')).map((v) => ({
    value: v,
    label: str(get(labels, v)) || v,
  }))
  const dataEntry = clone(snapshot.data_entry)
  const board = mapAt(dataEntry, ['kanbans', target.kanbanId])
  const listed = board && get(board, 'columns') !== undefined
  const columns: Option[] = (board ? mapItems(get(board, 'columns')) : []).map((c) => ({
    value: str(get(c, 'value')) ?? '',
    label: str(get(c, 'label')) ?? '',
  }))

  const shown = listed ? matching(columns, value, name) : matching(values, value, name)
  if (shown) return { reason: 'exists', name: shown.label || shown.value }

  const draft: ConfigDraftBody = { base_version: snapshot.version }
  const known = matching(values, value, name)
  const column = known ?? { value, label: name }
  if (!known) {
    ensureList(prop, 'values').push(value)
    if (name !== value) set(ensureMap(prop, 'labels'), value, name)
    draft.schema = schema
  }
  if (board && listed) {
    ensureList(board, 'columns').push(newMap({ value: column.value, label: column.label }))
    draft.data_entry = dataEntry
  }
  return { value: column.value, draft }
}
