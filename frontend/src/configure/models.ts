/**
 * Screen models: what each Configure screen shows, read from the two trees.
 *
 * A screen never reads a tree directly. It reads one of these, so the names
 * the files use stay here and every screen talks about entity types, choice
 * lists and rules. A choice list, for one, is `types.<name>` in the data
 * model plus `styles.<name>` (its colours) in the screens file.
 */
import {
  bool,
  entries,
  get,
  getIn,
  isList,
  isMap,
  keys,
  mapItems,
  newMap,
  num,
  str,
  strList,
  type TreeMap,
  type TreePath,
  type TreeValue,
} from './tree'
import { builtinTypeLabel } from './names'

// --- Data model ---

export interface PropertyModel {
  name: string
  label: string
  /** The type as the file says it: `string`, `date`, or a choice list's name. */
  type: string
  /** The type as the screen says it: `Text`, `Choice list: Ticket status`. */
  typeLabel: string
  required: boolean
  /** Holds several values. */
  list: boolean
  unique: boolean
  description: string
  /** Set when the type is a choice list. */
  choiceList?: string
  /** Options written on the property itself rather than in a choice list. */
  inlineValues: string[]
  /** The value is computed, or the property runs a scan: shown, never edited here. */
  managed: boolean
}

export interface EntityTypeModel {
  name: string
  label: string
  plural: string
  idType: string
  idPrefix: string
  color: string
  borderColor: string
  description: string
  displayProperty: string
  properties: PropertyModel[]
  relationCount: number
}

export interface OptionModel {
  value: string
  label: string
  color?: string
}

export interface ChoiceListModel {
  name: string
  label: string
  options: OptionModel[]
  default?: string
  /** Every entity type property that uses it. */
  usedBy: { entityType: string; property: string }[]
}

export interface RelationModel {
  name: string
  label: string
  description: string
  from: string[]
  to: string[]
  /** How the relation reads from the other end: `implemented by`. */
  inverse: string
  minOutgoing?: number
  maxOutgoing?: number
  minIncoming?: number
  maxIncoming?: number
}

export function entityTypeNames(schema: TreeValue | undefined): string[] {
  return keys(get(schema, 'entities'))
}

export function entityTypeNode(schema: TreeValue | undefined, name: string): TreeMap | undefined {
  const node = getIn(schema, ['entities', name])
  return isMap(node) ? node : undefined
}

export function entityTypeLabel(schema: TreeValue | undefined, name: string): string {
  return str(get(entityTypeNode(schema, name), 'label')) || name
}

/** Choice lists are the custom types that list their options. */
export function choiceListNames(schema: TreeValue | undefined): string[] {
  return entries(get(schema, 'types'))
    .filter(([, def]) => isList(get(def, 'values')))
    .map(([name]) => name)
}

export function choiceListLabel(name: string): string {
  return name
}

export function typeLabel(schema: TreeValue | undefined, type: string): string {
  const builtin = builtinTypeLabel(type)
  if (builtin) return builtin
  if (choiceListNames(schema).includes(type)) return `Choice list: ${choiceListLabel(type)}`
  if (type === 'enum') return 'Choice'
  return type
}

export function propertyModel(
  schema: TreeValue | undefined,
  name: string,
  def: TreeValue
): PropertyModel {
  const type = str(get(def, 'type')) || 'string'
  const inlineValues = strList(get(def, 'values'))
  const isChoice = choiceListNames(schema).includes(type)
  return {
    name,
    label: name,
    type,
    typeLabel: inlineValues.length ? 'Choice' : typeLabel(schema, type),
    required: bool(get(def, 'required')),
    list: bool(get(def, 'list')),
    unique: bool(get(def, 'unique')),
    description: str(get(def, 'description')) ?? '',
    choiceList: isChoice ? type : undefined,
    inlineValues,
    managed: get(def, 'computed') !== undefined || get(def, 'scan_cmd') !== undefined,
  }
}

export function propertiesOf(schema: TreeValue | undefined, entityType: string): PropertyModel[] {
  return entries(get(entityTypeNode(schema, entityType), 'properties')).map(([name, def]) =>
    propertyModel(schema, name, def)
  )
}

export function entityTypeModel(
  schema: TreeValue | undefined,
  name: string
): EntityTypeModel | undefined {
  const node = entityTypeNode(schema, name)
  if (!node) return undefined
  const label = str(get(node, 'label')) || name
  return {
    name,
    label,
    plural: str(get(node, 'label_plural')) || str(get(node, 'plural')) || label,
    idType: str(get(node, 'id_type')) || 'short',
    idPrefix: str(get(node, 'id_prefix')) ?? '',
    color: str(get(node, 'color')) ?? '',
    borderColor: str(get(node, 'border_color')) ?? '',
    description: str(get(node, 'description')) ?? '',
    displayProperty: str(get(node, 'display_property')) ?? '',
    properties: propertiesOf(schema, name),
    relationCount: relations(schema).filter((r) => r.from.includes(name) || r.to.includes(name))
      .length,
  }
}

export function entityTypes(schema: TreeValue | undefined): EntityTypeModel[] {
  return entityTypeNames(schema)
    .map((name) => entityTypeModel(schema, name))
    .filter((m): m is EntityTypeModel => m !== undefined)
}

/** The colours of a choice list's options, from the screens file. */
export function optionColors(
  dataEntry: TreeValue | undefined,
  list: string
): Record<string, string> {
  const colors: Record<string, string> = {}
  for (const [value, color] of entries(getIn(dataEntry, ['styles', list]))) {
    const c = str(color)
    if (c) colors[value] = c
  }
  return colors
}

export function choiceListModel(
  schema: TreeValue | undefined,
  dataEntry: TreeValue | undefined,
  name: string
): ChoiceListModel | undefined {
  const def = getIn(schema, ['types', name])
  if (!isMap(def) || !isList(get(def, 'values'))) return undefined
  const labels = get(def, 'labels')
  const colors = optionColors(dataEntry, name)
  const usedBy: ChoiceListModel['usedBy'] = []
  for (const type of entityTypeNames(schema)) {
    for (const p of propertiesOf(schema, type)) {
      if (p.type === name) usedBy.push({ entityType: type, property: p.name })
    }
  }
  return {
    name,
    label: choiceListLabel(name),
    options: strList(get(def, 'values')).map((value) => ({
      value,
      label: str(get(labels, value)) || value,
      color: colors[value],
    })),
    default: str(get(def, 'default')),
    usedBy,
  }
}

export function choiceLists(
  schema: TreeValue | undefined,
  dataEntry: TreeValue | undefined
): ChoiceListModel[] {
  return choiceListNames(schema)
    .map((name) => choiceListModel(schema, dataEntry, name))
    .filter((m): m is ChoiceListModel => m !== undefined)
}

function inverseOf(def: TreeValue): string {
  const inverse = get(def, 'inverse')
  if (isMap(inverse)) return str(get(inverse, 'label')) || str(get(inverse, 'id')) || ''
  return str(inverse) ?? ''
}

export function relationModel(
  schema: TreeValue | undefined,
  name: string
): RelationModel | undefined {
  const def = getIn(schema, ['relations', name])
  if (!isMap(def)) return undefined
  return {
    name,
    label: str(get(def, 'label')) || name,
    description: str(get(def, 'description')) ?? '',
    from: strList(get(def, 'from')),
    to: strList(get(def, 'to')),
    inverse: inverseOf(def),
    minOutgoing: num(get(def, 'min_outgoing')),
    maxOutgoing: num(get(def, 'max_outgoing')),
    minIncoming: num(get(def, 'min_incoming')),
    maxIncoming: num(get(def, 'max_incoming')),
  }
}

export function relations(schema: TreeValue | undefined): RelationModel[] {
  return keys(get(schema, 'relations'))
    .map((name) => relationModel(schema, name))
    .filter((m): m is RelationModel => m !== undefined)
}

/** `A ticket implements a feature`, with the type labels of the draft. */
export function relationSentence(
  schema: TreeValue | undefined,
  r: RelationModel,
  reverse = false
): string {
  const names = (types: string[]) =>
    types.map((t) => entityTypeLabel(schema, t)).join(' or ') || '…'
  return reverse
    ? `A ${names(r.to)} is ${r.inverse || '…'} a ${names(r.from)}`
    : `A ${names(r.from)} ${r.label || '…'} a ${names(r.to)}`
}

// --- Rules and automations ---

export interface RuleModel {
  index: number
  name: string
  description: string
  entityType: string
  when: string[]
  then: string[]
  whenCondition: string
  thenCondition: string
  severity: string
  /** Checks relation counts rather than property values. */
  checksRelations: boolean
  /** Runs a script: shown, never edited here. */
  usesScript: boolean
}

export function rules(schema: TreeValue | undefined): RuleModel[] {
  const list = get(schema, 'validations')
  if (!isList(list)) return []
  return list.flatMap((rule, index) => {
    if (!isMap(rule)) return []
    return [
      {
        index,
        name: str(get(rule, 'name')) ?? '',
        description: str(get(rule, 'description')) ?? '',
        entityType: str(get(rule, 'entity_type')) ?? '',
        when: strList(get(rule, 'when')),
        then: strList(get(rule, 'then')),
        whenCondition: str(get(rule, 'when_condition')) ?? '',
        thenCondition: str(get(rule, 'then_condition')) ?? '',
        severity: str(get(rule, 'severity')) || 'error',
        checksRelations: get(rule, 'relations') !== undefined,
        usesScript: get(rule, 'lua') !== undefined || get(rule, 'lua_file') !== undefined,
      },
    ]
  })
}

export function ruleTitle(rule: Pick<RuleModel, 'name' | 'description' | 'index'>): string {
  return rule.description || (rule.name ? rule.name : `Rule ${rule.index + 1}`)
}

export interface ActionModel {
  /** set, create_relation, create_entity, script or other. */
  kind: string
  summary: string
  /** Runs a script or holds settings Configure does not edit. */
  locked: boolean
}

export interface AutomationModel {
  index: number
  name: string
  description: string
  entityTypes: string[]
  /** The trigger in words: `Status becomes Planning`. */
  trigger: string
  property: string
  becomes: string
  created: boolean
  actions: ActionModel[]
  /** Some action is locked, so the whole automation is shown read-only. */
  readOnly: boolean
}

const LOCKED_ACTION_KEYS = ['lua', 'lua_file', 'capabilities', 'allow_acl_bypass']

function actionModel(action: TreeValue): ActionModel {
  const locked = LOCKED_ACTION_KEYS.some((k) => get(action, k) !== undefined)
  if (locked) return { kind: 'script', summary: 'Run a script', locked: true }
  const set = str(get(action, 'set'))
  if (set !== undefined) {
    return {
      kind: 'set',
      summary: `Set ${set} to ${str(get(action, 'value')) ?? '…'}`,
      locked: false,
    }
  }
  const rel = get(action, 'create_relation')
  if (isMap(rel)) {
    return {
      kind: 'create_relation',
      summary: `Link by ${str(get(rel, 'relation')) ?? '…'} to ${str(get(rel, 'to')) ?? '…'}`,
      locked: false,
    }
  }
  const ent = get(action, 'create_entity')
  if (isMap(ent)) {
    const template = get(ent, 'template') !== undefined
    return {
      kind: 'create_entity',
      summary: `Create a ${str(get(ent, 'type')) ?? 'record'}${
        str(get(ent, 'relation')) ? `, linked by ${str(get(ent, 'relation')) ?? ''}` : ''
      }${str(get(ent, 'if_exists')) === 'skip' ? ', unless one exists' : ''}`,
      locked: template,
    }
  }
  return { kind: 'other', summary: keys(action)[0] ?? 'action', locked: true }
}

export function automations(schema: TreeValue | undefined): AutomationModel[] {
  const list = get(schema, 'automations')
  if (!isList(list)) return []
  return list.flatMap((auto, index) => {
    if (!isMap(auto)) return []
    const on = get(auto, 'on')
    const property = str(get(on, 'property')) ?? ''
    const becomes = str(get(on, 'becomes')) ?? ''
    const created = bool(get(on, 'created'))
    const actions = mapItems(get(auto, 'do')).map(actionModel)
    let trigger = 'A record changes'
    if (created) trigger = 'A record is created'
    else if (property && becomes) trigger = `${property} becomes ${becomes}`
    else if (property) trigger = `${property} changes`
    else if (get(on, 'relation_created') !== undefined) trigger = 'A link is added'
    else if (get(on, 'relation_removed') !== undefined) trigger = 'A link is removed'
    return [
      {
        index,
        name: str(get(auto, 'name')) ?? '',
        description: str(get(auto, 'description')) ?? '',
        entityTypes: strList(get(on, 'entity')),
        trigger,
        property,
        becomes,
        created,
        actions,
        readOnly: actions.some((a) => a.locked) || get(on, 'faces') !== undefined,
      },
    ]
  })
}

export function automationTitle(
  a: Pick<AutomationModel, 'name' | 'description' | 'index'>
): string {
  return a.description || (a.name ? a.name : `Automation ${a.index + 1}`)
}

// --- Screens ---

export interface FieldModel {
  /** The field's place in the form's list. */
  index: number
  property: string
  label: string
  help: string
  placeholder: string
  hidden: boolean
  /** Columns out of 12. */
  span?: number
  widget: string
  /** Allowed changes per value, for a status-like field. */
  transitions: Record<string, string[]>
}

export interface FormModel {
  name: string
  title: string
  entityType: string
  mode: string
  description: string
  body: boolean
  fields: FieldModel[]
  /** Linked records offered on the form. */
  relations: { index: number; relation: string; label: string; widget: string; direction: string }[]
  /** A wizard form: its fields live in steps, which are not edited here yet. */
  hasSteps: boolean
}

function fieldModel(item: TreeMap, index: number): FieldModel {
  const transitions: Record<string, string[]> = {}
  for (const [from, to] of entries(get(item, 'transitions'))) transitions[from] = strList(to)
  return {
    index,
    property: str(get(item, 'property')) ?? '',
    label: str(get(item, 'label')) ?? '',
    help: str(get(item, 'help')) ?? '',
    placeholder: str(get(item, 'placeholder')) ?? '',
    hidden: bool(get(item, 'hidden')),
    span: num(get(item, 'span')),
    widget: str(get(item, 'widget')) ?? '',
    transitions,
  }
}

export function formModel(dataEntry: TreeValue | undefined, name: string): FormModel | undefined {
  const form = getIn(dataEntry, ['forms', name])
  if (!isMap(form)) return undefined
  const fieldList = get(form, 'fields')
  return {
    name,
    title: str(get(form, 'title')) || name,
    entityType: str(get(form, 'entity_type')) ?? '',
    mode: str(get(form, 'mode')) || 'create',
    description: str(get(form, 'description')) ?? '',
    body: bool(get(form, 'body')),
    fields: isList(fieldList)
      ? fieldList.flatMap((f, i) => (isMap(f) ? [fieldModel(f, i)] : []))
      : [],
    relations: (() => {
      const list = get(form, 'relations')
      if (!isList(list)) return []
      return list.flatMap((r, index) =>
        isMap(r)
          ? [
              {
                index,
                relation: str(get(r, 'relation')) ?? '',
                label: str(get(r, 'label')) ?? '',
                widget: str(get(r, 'widget')) ?? '',
                direction: str(get(r, 'direction')) || 'outgoing',
              },
            ]
          : []
      )
    })(),
    hasSteps: get(form, 'steps') !== undefined,
  }
}

export function forms(dataEntry: TreeValue | undefined): FormModel[] {
  return keys(get(dataEntry, 'forms'))
    .map((name) => formModel(dataEntry, name))
    .filter((m): m is FormModel => m !== undefined)
}

export interface ColumnModel {
  index: number
  property: string
  relation: string
  label: string
  sortable: boolean
  link: string
}

export interface ListModel {
  name: string
  title: string
  entityType: string
  description: string
  header: string
  footer: string
  pageSize?: number
  columns: ColumnModel[]
  sort: SortKeyModel[]
  filterControls: FilterControlModel[]
  fixedFilters: FixedFilterModel[]
  condition: string
  queryScope: string
  createForm: string
  editForm: string
  detailView: string
}

/** One key of a `sort:` list. */
export interface SortKeyModel {
  index: number
  property: string
  direction: string
}

/** One filter people can set on a list or board (`filter_controls:`). */
export interface FilterControlModel {
  index: number
  property: string
  relation: string
  /** For a relation: `outgoing` (the default) or `incoming`. */
  direction: string
  label: string
}

/** One fixed filter a list or board always applies (`filters:`). */
export interface FixedFilterModel {
  index: number
  property: string
  operator: string
  value: string
}

export function sortKeysOf(value: TreeValue | undefined): SortKeyModel[] {
  return isList(value)
    ? value.flatMap((item, index) =>
        isMap(item)
          ? [
              {
                index,
                property: str(get(item, 'property')) ?? '',
                direction: str(get(item, 'direction')) || 'asc',
              },
            ]
          : []
      )
    : []
}

export function filterControlsOf(value: TreeValue | undefined): FilterControlModel[] {
  return isList(value)
    ? value.flatMap((item, index) =>
        isMap(item)
          ? [
              {
                index,
                property: str(get(item, 'property')) ?? '',
                relation: str(get(item, 'relation')) ?? '',
                direction: str(get(item, 'direction')) || 'outgoing',
                label: str(get(item, 'label')) ?? '',
              },
            ]
          : []
      )
    : []
}

export function fixedFiltersOf(value: TreeValue | undefined): FixedFilterModel[] {
  return isList(value)
    ? value.flatMap((item, index) =>
        isMap(item)
          ? [
              {
                index,
                property: str(get(item, 'property')) ?? '',
                operator: str(get(item, 'operator')) || '=',
                value: scalarText(get(item, 'value')),
              },
            ]
          : []
      )
    : []
}

/**
 * The card fields after choosing `properties` in a picker that shows only
 * property fields. A field that stays keeps its item as it was (label,
 * direction and other settings); a relation field the picker does not show
 * always stays; new properties are added at the end.
 */
export function chooseCardFields(fields: TreeValue | undefined, properties: string[]): TreeValue[] {
  const items = isList(fields) ? fields : []
  const kept = items.filter((item) => {
    const property = str(get(item, 'property'))
    return property ? properties.includes(property) : true
  })
  const present = new Set(kept.map((item) => str(get(item, 'property'))))
  return [
    ...kept,
    ...properties.filter((p) => !present.has(p)).map((property) => newMap({ property })),
  ]
}

/**
 * A property default as the text the drawer edits. A list or mapping default
 * has no text form; the drawer leaves it as it is.
 */
export function defaultText(value: TreeValue | undefined): string | undefined {
  if (value === undefined || value === null) return ''
  if (typeof value === 'object') return undefined
  return String(value)
}

/**
 * The default to save for `text`, typed for a property of type `type`: a
 * number for a number type, a boolean for a yes/no type. Text that does not
 * read as that type is saved as written, so the server reports it.
 */
export function typedDefault(text: string, type: string): TreeValue | undefined {
  const t = text.trim()
  if (!t) return undefined
  if ((type === 'integer' || type === 'number' || type === 'float') && /^-?\d+(\.\d+)?$/.test(t))
    return Number(t)
  if (type === 'boolean' && (t === 'true' || t === 'false')) return t === 'true'
  return t
}

/** A scalar as text: numbers and booleans in YAML are still filter values. */
function scalarText(value: TreeValue | undefined): string {
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return str(value) ?? ''
}

/** The names of the query scopes an entity type declares. */
export function queryScopeNames(schema: TreeValue | undefined, entityType: string): string[] {
  return keys(getIn(schema, ['entities', entityType, 'query_scopes']))
}

/**
 * The relations that touch an entity type: `outgoing` where it is a source,
 * `incoming` where it is a target. A relation that lists no types touches
 * every type.
 */
export function relationsOfType(
  schema: TreeValue | undefined,
  entityType: string
): { name: string; label: string; direction: 'outgoing' | 'incoming' }[] {
  return relations(schema).flatMap((r) => {
    const out: { name: string; label: string; direction: 'outgoing' | 'incoming' }[] = []
    if (!r.from.length || r.from.includes(entityType))
      out.push({ name: r.name, label: r.label, direction: 'outgoing' })
    if (!r.to.length || r.to.includes(entityType))
      out.push({ name: r.name, label: r.inverse || r.label, direction: 'incoming' })
    return out
  })
}

function columnModel(item: TreeMap, index: number): ColumnModel {
  return {
    index,
    property: str(get(item, 'property')) ?? '',
    relation: str(get(item, 'relation')) ?? '',
    label: str(get(item, 'label')) ?? '',
    sortable: bool(get(item, 'sortable')),
    link: str(get(item, 'link')) ?? '',
  }
}

function columnsOf(list: TreeValue | undefined): ColumnModel[] {
  return isList(list) ? list.flatMap((c, i) => (isMap(c) ? [columnModel(c, i)] : [])) : []
}

export function columnTitle(c: ColumnModel): string {
  return c.label || c.property || c.relation || 'column'
}

export function listModel(dataEntry: TreeValue | undefined, name: string): ListModel | undefined {
  const list = getIn(dataEntry, ['lists', name])
  if (!isMap(list)) return undefined
  return {
    name,
    title: str(get(list, 'title')) || name,
    entityType: str(get(list, 'entity_type')) ?? '',
    description: str(get(list, 'description')) ?? '',
    header: str(get(list, 'header')) ?? '',
    footer: str(get(list, 'footer')) ?? '',
    pageSize: num(get(list, 'page_size')),
    columns: columnsOf(get(list, 'columns')),
    sort: sortKeysOf(get(list, 'sort')),
    filterControls: filterControlsOf(get(list, 'filter_controls')),
    fixedFilters: fixedFiltersOf(get(list, 'filters')),
    condition: str(get(list, 'condition')) ?? '',
    queryScope: str(get(list, 'query_scope')) ?? '',
    createForm: str(get(list, 'create_form')) ?? '',
    editForm: str(get(list, 'edit_form')) ?? '',
    detailView: str(get(list, 'detail_view')) ?? '',
  }
}

export function lists(dataEntry: TreeValue | undefined): ListModel[] {
  return keys(get(dataEntry, 'lists'))
    .map((name) => listModel(dataEntry, name))
    .filter((m): m is ListModel => m !== undefined)
}

export interface BoardModel {
  name: string
  title: string
  entityType: string
  columnProperty: string
  swimlaneProperty: string
  /** Columns the board lists by hand. Empty means one per option. */
  columns: { index: number; value: string; label: string }[]
  cardFields: string[]
  filterControls: FilterControlModel[]
  fixedFilters: FixedFilterModel[]
  queryScope: string
}

export function boardModel(dataEntry: TreeValue | undefined, name: string): BoardModel | undefined {
  const board = getIn(dataEntry, ['kanbans', name])
  if (!isMap(board)) return undefined
  const columns = get(board, 'columns')
  return {
    name,
    title: str(get(board, 'title')) || name,
    entityType: str(get(board, 'entity_type')) ?? '',
    columnProperty: str(get(board, 'column_property')) ?? '',
    swimlaneProperty: str(get(board, 'swimlane_property')) ?? '',
    columns: isList(columns)
      ? columns.flatMap((c, index) =>
          isMap(c)
            ? [{ index, value: str(get(c, 'value')) ?? '', label: str(get(c, 'label')) ?? '' }]
            : []
        )
      : [],
    cardFields: mapItems(getIn(board, ['card', 'fields']))
      .map((f) => str(get(f, 'property')) ?? '')
      .filter(Boolean),
    filterControls: filterControlsOf(get(board, 'filter_controls')),
    fixedFilters: fixedFiltersOf(get(board, 'filters')),
    queryScope: str(get(board, 'query_scope')) ?? '',
  }
}

export function boards(dataEntry: TreeValue | undefined): BoardModel[] {
  return keys(get(dataEntry, 'kanbans'))
    .map((name) => boardModel(dataEntry, name))
    .filter((m): m is BoardModel => m !== undefined)
}

export interface CardModel {
  index: number
  title: string
  query: string
  display: string
  groupBy: string
  limit?: number
  /** Only some people see it: shown, never edited here. */
  restricted: boolean
}

export function dashboardModel(dataEntry: TreeValue | undefined) {
  const dash = get(dataEntry, 'dashboard')
  const cards = get(dash, 'cards')
  return {
    title: str(get(dash, 'title')) ?? '',
    description: str(get(dash, 'description')) ?? '',
    cards: isList(cards)
      ? cards.flatMap((c, index): CardModel[] =>
          isMap(c)
            ? [
                {
                  index,
                  title: str(get(c, 'title')) ?? '',
                  query: str(get(c, 'query')) ?? '',
                  display: str(get(c, 'display')) || 'count',
                  groupBy: str(get(c, 'group_by')) ?? '',
                  limit: num(get(c, 'limit')),
                  restricted: get(c, 'permission') !== undefined,
                },
              ]
            : []
        )
      : [],
  }
}

// --- Navigation ---

/** The kinds of page a navigation entry can open, in the order they are checked. */
const NAV_KINDS: { key: string; label: string; icon: string }[] = [
  { key: 'dashboard', label: 'Dashboard', icon: 'dashboard' },
  { key: 'list', label: 'List', icon: 'list' },
  { key: 'kanban', label: 'Board', icon: 'kanban' },
  { key: 'calendar', label: 'Calendar', icon: 'calendar' },
  { key: 'gantt', label: 'Timeline', icon: 'gantt' },
  { key: 'document', label: 'Document', icon: 'document' },
  { key: 'page', label: 'Page', icon: '' },
  { key: 'entities', label: 'Records', icon: 'file' },
  { key: 'search', label: 'Search', icon: 'search' },
  { key: 'settings', label: 'Settings', icon: 'settings' },
  { key: 'action', label: 'Action', icon: 'zap' },
]

export interface NavEntryModel {
  index: number
  /** Where the entry sits in the screens tree. */
  path: TreePath
  label: string
  kind: string
  kindKey: string
  target: string
  icon: string
  /** Gated by a permission or runs an action: shown, never edited here. */
  locked: boolean
}

export interface NavGroupModel {
  /** Index in the navigation list; -1 for a run of loose entries outside any group. */
  index: number
  label: string
  entries: NavEntryModel[]
  locked: boolean
  /** The group fills itself from the rows of a list (`items_from:`). */
  fromList: boolean
  /** The list that holds the entries, and where the first of them sits in it. */
  itemsPath: TreePath
  offset: number
}

export interface SpaceModel {
  /** Index in `spaces`, or -1 for a project without spaces. */
  index: number
  id: string
  label: string
  icon: string
  create: string[]
  groups: NavGroupModel[]
}

export function navEntryModel(item: TreeMap, index: number, path: TreePath): NavEntryModel {
  const kind = NAV_KINDS.find((k) => get(item, k.key) !== undefined)
  const raw = kind ? get(item, kind.key) : undefined
  const target = typeof raw === 'string' ? raw : ''
  return {
    index,
    path,
    label: str(get(item, 'label')) || (target ? target : (kind?.label ?? 'Entry')),
    kind: kind?.label ?? 'Link',
    kindKey: kind?.key ?? '',
    target,
    icon: str(get(item, 'icon')) || kind?.icon || '',
    locked: get(item, 'permission') !== undefined || get(item, 'action') !== undefined,
  }
}

/**
 * The groups of a navigation list. Loose entries (not in a group) collect in
 * one unnamed group per run, so the screen can show and reorder them too.
 */
export function navGroups(nav: TreeValue | undefined, navPath: TreePath): NavGroupModel[] {
  if (!isList(nav)) return []
  const groups: NavGroupModel[] = []
  let loose: NavGroupModel | undefined
  nav.forEach((item, index) => {
    if (!isMap(item)) return
    if (get(item, 'group') !== undefined) {
      loose = undefined
      groups.push({
        index,
        label: str(get(item, 'group')) ?? '',
        entries: mapItems(get(item, 'items')).map((child, i) =>
          navEntryModel(child, i, [...navPath, index, 'items', i])
        ),
        locked: get(item, 'permission') !== undefined,
        fromList: get(item, 'items_from') !== undefined,
        itemsPath: [...navPath, index, 'items'],
        offset: 0,
      })
      return
    }
    if (!loose) {
      loose = {
        index: -1,
        label: '',
        entries: [],
        locked: false,
        fromList: false,
        itemsPath: navPath,
        offset: index,
      }
      groups.push(loose)
    }
    loose.entries.push(navEntryModel(item, index, [...navPath, index]))
  })
  return groups
}

export function spaces(dataEntry: TreeValue | undefined): SpaceModel[] {
  const list = get(dataEntry, 'spaces')
  if (isList(list) && list.length) {
    return list.flatMap((s, index): SpaceModel[] =>
      isMap(s)
        ? [
            {
              index,
              id: str(get(s, 'id')) ?? String(index),
              label: str(get(s, 'label')) || (str(get(s, 'id')) ?? ''),
              icon: str(get(s, 'icon')) ?? '',
              create: strList(get(s, 'create')),
              groups: navGroups(get(s, 'navigation'), ['spaces', index, 'navigation']),
            },
          ]
        : []
    )
  }
  return [
    {
      index: -1,
      id: '',
      label: str(getIn(dataEntry, ['app', 'name'])) || 'Navigation',
      icon: '',
      create: [],
      groups: navGroups(get(dataEntry, 'navigation'), ['navigation']),
    },
  ]
}
