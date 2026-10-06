/**
 * Keeps references to a property pointing at it when it is renamed.
 *
 * Forms, lists and boards name properties of their entity type. Renaming a
 * property in the data model without following those would break every
 * screen that shows it, and the server would refuse the save. So the draft
 * renames the references along with the property.
 */
import {
  entries,
  get,
  getIn,
  isMap,
  mapItems,
  set,
  str,
  strList,
  type TreeMap,
  type TreeValue,
} from './tree'

function renameIn(item: TreeValue | undefined, key: string, from: string, to: string): void {
  if (isMap(item) && str(get(item, key)) === from) set(item, key, to)
}

function renameEach(list: TreeValue | undefined, key: string, from: string, to: string): void {
  for (const item of mapItems(list)) renameIn(item, key, from, to)
}

function entityTypesOf(item: TreeValue | undefined): string[] {
  return strList(get(item, 'entity_type'))
}

/** Renames a property's references in the screens file, in place. */
export function renameScreenReferences(
  dataEntry: TreeMap,
  entityType: string,
  from: string,
  to: string
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
    if (!entityTypesOf(board).includes(entityType)) continue
    renameIn(board, 'column_property', from, to)
    renameIn(board, 'swimlane_property', from, to)
    renameEach(getIn(board, ['card', 'fields']), 'property', from, to)
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
