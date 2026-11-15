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
export type AddColumnRefusal =
  { reason: 'no_name' } | { reason: 'exists'; name: string } | { reason: 'unsupported' }

export type AddColumnEdit = { value: string; draft: ConfigDraftBody } | AddColumnRefusal

/**
 * The draft that adds a column labelled `label` to the board: the value
 * appended to the property's `values:`, a `labels:` entry when the label is
 * not the value itself, and an item on the board's `columns:` when it lists
 * them. The snapshot is not changed.
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
  const dataEntry = clone(snapshot.data_entry)
  const board = mapAt(dataEntry, ['kanbans', target.kanbanId])
  const columns = board ? mapItems(get(board, 'columns')) : []
  const taken = [
    ...strList(get(prop, 'values')).map((v) => ({ value: v, label: str(get(labels, v)) || v })),
    ...columns.map((c) => ({
      value: str(get(c, 'value')) ?? '',
      label: str(get(c, 'label')) ?? '',
    })),
  ]
  const clash = taken.find((c) => c.value === value || c.label.toLowerCase() === name.toLowerCase())
  if (clash) return { reason: 'exists', name: clash.label || clash.value }

  ensureList(prop, 'values').push(value)
  if (name !== value) set(ensureMap(prop, 'labels'), value, name)

  const draft: ConfigDraftBody = { base_version: snapshot.version, schema }
  if (board && get(board, 'columns') !== undefined) {
    ensureList(board, 'columns').push(newMap({ value, label: name }))
    draft.data_entry = dataEntry
  }
  return { value, draft }
}
