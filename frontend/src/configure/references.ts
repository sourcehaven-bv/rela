/**
 * Keeps references to a property pointing at it when it is renamed.
 *
 * Forms, lists and boards name properties of their entity type. Renaming a
 * property in the data model without following those would break every
 * screen that shows it, and the server would refuse the save. So the draft
 * renames the references along with the property.
 */
import { entries, get, getIn, isMap, mapItems, set, str, strList } from './tree'
import type { TreeMap, TreeValue } from '@/api/configure'

function renameIn(item: TreeValue | undefined, key: string, from: string, to: string): void {
  if (isMap(item) && str(get(item, key)) === from) set(item, key, to)
}

function renameEach(list: TreeValue | undefined, key: string, from: string, to: string): void {
  for (const item of mapItems(list)) renameIn(item, key, from, to)
}

function entityTypesOf(item: TreeValue | undefined): string[] {
  return strList(get(item, 'entity_type'))
}

/**
 * The entity types at the other end of a board card field's relation, as seen
 * from the board's type: `from` types for an incoming field, `to` types for an
 * outgoing one. Without a direction it follows the side the board is on, as
 * the server infers it.
 */
function relatedTypesOf(
  schema: TreeValue | undefined,
  field: TreeValue | undefined,
  boardTypes: string[]
): string[] {
  const rel = getIn(schema, ['relations', str(get(field, 'relation')) ?? ''])
  const fromTypes = strList(get(rel, 'from'))
  const toTypes = strList(get(rel, 'to'))
  const direction = str(get(field, 'direction'))
  if (direction === 'incoming') return fromTypes
  if (direction === 'outgoing') return toTypes
  // As the server's InferDirection: incoming only when the board is on the
  // `to` side and not the `from` side; otherwise outgoing.
  const incoming =
    !boardTypes.some((t) => fromTypes.includes(t)) && boardTypes.some((t) => toTypes.includes(t))
  return incoming ? fromTypes : toTypes
}

/**
 * Renames a board's card-field property references. A field with a relation
 * names a property of the related entities (TKT-8Z2LT3), so it follows a
 * rename on the related type rather than on the board's own type.
 */
function renameCardFields(
  board: TreeValue | undefined,
  schema: TreeValue | undefined,
  entityType: string,
  from: string,
  to: string
): void {
  const boardTypes = entityTypesOf(board)
  for (const field of mapItems(getIn(board, ['card', 'fields']))) {
    const owners = str(get(field, 'relation'))
      ? relatedTypesOf(schema, field, boardTypes)
      : boardTypes
    if (owners.includes(entityType)) renameIn(field, 'property', from, to)
  }
}

/**
 * Renames a property's references in the screens file, in place. `schema` is
 * the data model the screens refer to; a board field that reads a property
 * through a relation needs it to know which type that property belongs to.
 */
export function renameScreenReferences(
  dataEntry: TreeMap,
  entityType: string,
  from: string,
  to: string,
  schema: TreeValue | undefined
): void {
  for (const [, form] of entries(get(dataEntry, 'forms'))) {
    if (!entityTypesOf(form).includes(entityType)) continue
    renameEach(get(form, 'fields'), 'property', from, to)
    for (const step of mapItems(get(form, 'steps')))
      renameEach(get(step, 'fields'), 'property', from, to)
  }
  for (const [, list] of entries(get(dataEntry, 'lists'))) {
    if (!entityTypesOf(list).includes(entityType)) continue
    renameEach(get(list, 'columns'), 'property', from, to)
    renameEach(get(list, 'filter_controls'), 'property', from, to)
    renameEach(get(list, 'sort'), 'property', from, to)
    renameIn(list, 'group_by', from, to)
    renameIn(get(list, 'group_by'), 'property', from, to)
  }
  for (const [, board] of entries(get(dataEntry, 'kanbans'))) {
    renameCardFields(board, schema, entityType, from, to)
    if (!entityTypesOf(board).includes(entityType)) continue
    renameIn(board, 'column_property', from, to)
    renameIn(board, 'swimlane_property', from, to)
    renameEach(get(board, 'column_summary'), 'property', from, to)
    renameEach(get(board, 'filter_controls'), 'property', from, to)
  }
}

/** Renames a property's references inside its own entity type, in place. */
export function renameSchemaReferences(
  schema: TreeMap,
  entityType: string,
  from: string,
  to: string
): void {
  const type = getIn(schema, ['entities', entityType])
  if (!isMap(type)) return
  renameIn(type, 'display_property', from, to)
  renameEach(get(type, 'default_sort'), 'property', from, to)
  for (const auto of mapItems(get(schema, 'automations'))) {
    const on = get(auto, 'on')
    if (!strList(get(on, 'entity')).includes(entityType)) continue
    renameIn(on, 'property', from, to)
    renameEach(get(auto, 'do'), 'set', from, to)
  }
}
