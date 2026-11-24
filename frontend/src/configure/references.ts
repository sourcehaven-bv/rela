/**
 * Keeps references to a property pointing at it when it is renamed.
 *
 * Forms, lists, boards and automations name properties of their entity type. Renaming a
 * property in the data model without following those would break every
 * screen that shows it, and the server would refuse the save. So the draft
 * renames the references along with the property.
 */
import type { TreeMap, TreeValue } from '@/api/configure'
import { filterProperty, triggerLocked } from './models'
import { entries, get, getIn, has, isMap, mapItems, renameKey, set, str, strList } from './tree'

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

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/**
 * Renames `entity.<from>`, `new.<from>` and `old.<from>` in a text: a
 * `{{new.x}}` placeholder, a `where:` clause or a `when:` expression.
 */
function renameInText(text: string, from: string, to: string): string {
  const re = new RegExp(`\\b(entity|new|old)\\.${escapeRegExp(from)}(?![A-Za-z0-9_-])`, 'g')
  return text.replace(re, `$1.${to}`)
}

/** Renames the property a `where:` filter clause tests (`effort>0`). */
function renameFilter(clause: TreeValue, from: string, to: string): TreeValue {
  if (typeof clause !== 'string' || filterProperty(clause) !== from) return clause
  return clause.replace(new RegExp(`^(\\s*)${escapeRegExp(from)}`), `$1${to}`)
}

/** Applies renameInText to every text in a tree, in place. */
function renameTexts(value: TreeValue | undefined, from: string, to: string): void {
  if (isMap(value)) {
    for (const entry of value.$m) {
      const v = entry[1]
      if (typeof v === 'string') entry[1] = renameInText(v, from, to)
      else renameTexts(v, from, to)
    }
  } else if (Array.isArray(value)) {
    value.forEach((v, i) => {
      if (typeof v === 'string') value[i] = renameInText(v, from, to)
      else renameTexts(v, from, to)
    })
  }
}

/**
 * Renames a property's references in the flows.yaml triggers, in place. On
 * a trigger of its entity type: `on.property`, the keys `set` writes,
 * `notify.to.property`, and the property in every placeholder, `where:`
 * clause and `when:` expression. On any trigger: the keys a `create` of
 * that type writes. A trigger the Configure space does not edit is left
 * alone; the save then reports it.
 */
export function renameTriggerReferences(
  flows: TreeMap,
  entityType: string,
  from: string,
  to: string
): void {
  for (const [, rule] of entries(get(flows, 'triggers'))) {
    if (!isMap(rule) || triggerLocked(rule)) continue
    for (const output of mapItems(get(rule, 'outputs'))) {
      const create = get(output, 'create')
      const props = get(create, 'properties')
      if (str(get(create, 'type')) === entityType && isMap(props) && has(props, from))
        renameKey(props, from, to)
    }
    const on = get(rule, 'on')
    if (!strList(get(on, 'entity')).includes(entityType)) continue
    renameIn(on, 'property', from, to)
    renameTexts(on, from, to)
    const where = get(on, 'where')
    if (Array.isArray(where)) where.forEach((c, i) => (where[i] = renameFilter(c, from, to)))
    for (const output of mapItems(get(rule, 'outputs'))) {
      const props = getIn(output, ['set', 'properties'])
      if (isMap(props) && has(props, from)) renameKey(props, from, to)
      const setOut = get(output, 'set')
      const unset = get(setOut, 'unset')
      if (isMap(setOut) && Array.isArray(unset) && unset.includes(from)) {
        set(setOut, 'unset', unset.map((name) => (name === from ? to : name)))
      }
      const notifyTo = getIn(output, ['notify', 'to'])
      if (isMap(notifyTo)) renameIn(notifyTo, 'property', from, to)
      renameTexts(output, from, to)
      const check = get(output, 'check')
      if (isMap(check)) {
        const clause = get(check, 'where')
        if (typeof clause === 'string') set(check, 'where', renameFilter(clause, from, to))
      }
    }
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
}
