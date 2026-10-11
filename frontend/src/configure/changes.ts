/**
 * The draft's changes in the reader's terms, worked out by comparing the
 * trees the server sent with the draft's trees.
 *
 * Changes are grouped by the thing they change (an entity type, a choice
 * list, a form) and each one links back to the screen where it was made. The
 * wording never names a file or a key path: `Property Due`, `Option Blocked`,
 * `Order of fields`.
 */
import type { ConfigRename } from '@/api/configure'
import {
  entries,
  equal,
  get,
  getIn,
  isList,
  isMap,
  keys,
  str,
  strList,
  type TreeMap,
  type TreeValue,
} from './tree'
import {
  automationTitle,
  choiceListLabel,
  choiceListNames,
  columnTitle,
  entityTypeLabel,
  propertyModel,
  ruleTitle,
  typeLabel,
} from './models'
import { configureRoute } from './routes'

export type ChangeKind = 'added' | 'changed' | 'removed'

export interface ChangeItem {
  kind: ChangeKind
  label: string
  detail?: string
  before?: string
  after?: string
  /** The screen the change was made on. */
  to: string
}

export interface ChangeGroup {
  /** Stable across recomputes, for list keys. */
  key: string
  title: string
  items: ChangeItem[]
}

export interface DraftTrees {
  schema: TreeValue | undefined
  dataEntry: TreeValue | undefined
}

/** A scalar or a list of scalars as text, for a before/after pair. */
function show(value: TreeValue | undefined): string | undefined {
  if (value === undefined || value === null) return undefined
  if (isList(value)) {
    const items = strList(value)
    return items.length === value.length ? items.join(', ') : undefined
  }
  if (isMap(value)) return undefined
  if (value === true) return 'Yes'
  if (value === false) return 'No'
  return str(value)
}

/** What a setting is called on screen. Unknown keys show as written. */
const SETTING_LABELS: Record<string, string> = {
  label: 'Name',
  label_plural: 'Plural name',
  id_prefix: 'ID prefix',
  id_type: 'IDs',
  id_caps: 'ID letters',
  color: 'Colour',
  border_color: 'Border colour',
  display_property: 'Title property',
  description: 'Description',
  title: 'Title',
  entity_type: 'Entity type',
  page_size: 'Rows per page',
  create_form: 'Create form',
  edit_form: 'Edit form',
  column_property: 'Columns from',
  swimlane_property: 'Rows from',
  min_outgoing: 'At least',
  max_outgoing: 'At most',
  min_incoming: 'At least, the other way round',
  max_incoming: 'At most, the other way round',
  inverse: 'Name the other way round',
  from: 'From',
  to: 'To',
  severity: 'When broken',
  when: 'When',
  then: 'Then',
  when_condition: 'When',
  then_condition: 'Then',
  required: 'Required',
  list: 'Several values',
  unique: 'Unique',
  type: 'Type',
  default: 'Default',
  query: 'Shows',
  display: 'Shown as',
  group_by: 'Grouped by',
  limit: 'At most',
  body: 'Description editor',
  mode: 'Mode',
  header: 'Text above',
  footer: 'Text below',
  sort: 'Sorted by',
  filter_controls: 'Filters',
  card: 'Cards',
  help: 'Help text',
  placeholder: 'Placeholder',
  hidden: 'Hidden',
  span: 'Width',
  transitions: 'Allowed changes',
  icon: 'Icon',
  create: 'The New button creates',
  sortable: 'Sortable',
  link: 'Opens the record',
}

export function settingLabel(key: string): string {
  return SETTING_LABELS[key] ?? key
}

/** One change per differing key of two mappings, worded as settings. */
function settingChanges(
  before: TreeValue | undefined,
  after: TreeValue | undefined,
  to: string,
  skip: string[] = [],
  prefix = ''
): ChangeItem[] {
  const out: ChangeItem[] = []
  const all = [...keys(before)]
  for (const k of keys(after)) if (!all.includes(k)) all.push(k)
  for (const key of all) {
    if (skip.includes(key)) continue
    const b = get(before, key)
    const a = get(after, key)
    if (equal(b, a)) continue
    const label = prefix + settingLabel(key)
    const kind: ChangeKind = b === undefined ? 'added' : a === undefined ? 'removed' : 'changed'
    out.push({ kind, label, before: show(b), after: show(a), to })
  }
  return out
}

/** Whether the shared keys of two mappings appear in a different order. */
function reordered(beforeKeys: string[], afterKeys: string[]): boolean {
  const shared = afterKeys.filter((k) => beforeKeys.includes(k))
  const was = beforeKeys.filter((k) => shared.includes(k))
  return shared.join('\u0000') !== was.join('\u0000')
}

export interface ListDiff {
  added: TreeMap[]
  removed: TreeMap[]
  changed: [TreeMap, TreeMap][]
  reordered: boolean
}

/**
 * Compares two lists of mappings. Items are matched by `$i`, the index an
 * item had when the file was read, which the draft never changes; an item
 * without one is new.
 */
export function diffList(before: TreeValue | undefined, after: TreeValue | undefined): ListDiff {
  const was = isList(before) ? before : []
  const now = isList(after) ? after : []
  const out: ListDiff = { added: [], removed: [], changed: [], reordered: false }
  const seen = new Set<number>()
  const order: number[] = []
  for (const item of now) {
    if (!isMap(item)) continue
    const origin = item.$i
    const old = origin === undefined ? undefined : was[origin]
    if (origin === undefined || !isMap(old)) {
      out.added.push(item)
      continue
    }
    seen.add(origin)
    order.push(origin)
    if (!equal(old, item)) out.changed.push([old, item])
  }
  was.forEach((item, index) => {
    if (isMap(item) && !seen.has(index)) out.removed.push(item)
  })
  out.reordered = order.some((value, i) => i > 0 && value < order[i - 1])
  return out
}

// --- Data model ---

function propertyChanges(
  schemaBefore: TreeValue | undefined,
  schemaAfter: TreeValue | undefined,
  type: string,
  renames: ConfigRename[],
  to: string
): ChangeItem[] {
  const before = getIn(schemaBefore, ['entities', type, 'properties'])
  const after = getIn(schemaAfter, ['entities', type, 'properties'])
  const out: ChangeItem[] = []
  const renamedFrom = new Map<string, string>()
  for (const r of renames) if (r.entity_type === type) renamedFrom.set(r.to, r.from)
  const matched: string[] = []
  const afterOrder: string[] = []
  for (const [name, def] of entries(after)) {
    const origin = renamedFrom.get(name) ?? name
    const old = get(before, origin)
    const link = `${to}?property=${encodeURIComponent(name)}`
    const label = `Property ${name}`
    if (old === undefined) {
      const m = propertyModel(schemaAfter, name, def)
      out.push({
        kind: 'added',
        label,
        detail: `${m.typeLabel}${m.required ? ', required' : ', optional'}`,
        to: link,
      })
      continue
    }
    matched.push(origin)
    afterOrder.push(origin)
    if (origin !== name) {
      out.push({ kind: 'changed', label, detail: 'renamed', before: origin, after: name, to: link })
    }
    for (const change of settingChanges(old, def, link, ['type'])) {
      out.push({ ...change, label: `${settingLabel(keyOf(change.label))} of ${name}` })
    }
    const oldType = str(get(old, 'type')) ?? 'string'
    const newType = str(get(def, 'type')) ?? 'string'
    if (oldType !== newType) {
      out.push({
        kind: 'changed',
        label: `Type of ${name}`,
        before: typeLabel(schemaBefore, oldType),
        after: typeLabel(schemaAfter, newType),
        to: link,
      })
    }
  }
  for (const name of keys(before)) {
    if (!matched.includes(name)) out.push({ kind: 'removed', label: `Property ${name}`, to })
  }
  if (reordered(keys(before), afterOrder))
    out.push({ kind: 'changed', label: 'Order of properties', to })
  return out
}

/** settingChanges labels are the setting's label; this maps a label back to its key. */
function keyOf(label: string): string {
  const found = Object.entries(SETTING_LABELS).find(([, l]) => l === label)
  return found ? found[0] : label
}

function entityTypeChanges(b: DraftTrees, a: DraftTrees, renames: ConfigRename[]): ChangeGroup[] {
  const before = get(b.schema, 'entities')
  const after = get(a.schema, 'entities')
  const groups: ChangeGroup[] = []
  for (const name of keys(after)) {
    const to = configureRoute.entityType(name)
    const title = entityTypeLabel(a.schema, name)
    const old = get(before, name)
    if (old === undefined) {
      const count = keys(getIn(after, [name, 'properties'])).length
      groups.push({
        key: `entity:${name}`,
        title,
        items: [
          { kind: 'added', label: `Entity type ${title}`, detail: `${count} properties`, to },
        ],
      })
      continue
    }
    const items = [
      ...settingChanges(old, get(after, name), to, ['properties']),
      ...propertyChanges(b.schema, a.schema, name, renames, to),
    ]
    if (items.length) groups.push({ key: `entity:${name}`, title, items })
  }
  for (const name of keys(before)) {
    if (get(after, name) === undefined) {
      groups.push({
        key: `entity:${name}`,
        title: entityTypeLabel(b.schema, name),
        items: [
          {
            kind: 'removed',
            label: `Entity type ${entityTypeLabel(b.schema, name)}`,
            to: configureRoute.entityTypes(),
          },
        ],
      })
    }
  }
  return groups
}

function optionChanges(
  before: TreeValue | undefined,
  after: TreeValue | undefined,
  stylesBefore: TreeValue | undefined,
  stylesAfter: TreeValue | undefined,
  to: string
): ChangeItem[] {
  const was = strList(get(before, 'values'))
  const now = strList(get(after, 'values'))
  const out: ChangeItem[] = []
  for (const v of now) {
    if (!was.includes(v)) {
      out.push({ kind: 'added', label: `Option ${v}`, after: show(get(stylesAfter, v)), to })
    }
  }
  for (const v of was) if (!now.includes(v)) out.push({ kind: 'removed', label: `Option ${v}`, to })
  if (reordered(was, now)) out.push({ kind: 'changed', label: 'Order of options', to })
  for (const v of now) {
    if (!was.includes(v)) continue
    const b = str(get(stylesBefore, v))
    const c = str(get(stylesAfter, v))
    if (b !== c) {
      out.push({ kind: 'changed', label: `Colour of ${v}`, before: b && b, after: c && c, to })
    }
  }
  out.push(...settingChanges(before, after, to, ['values']))
  return out
}

function choiceListChanges(b: DraftTrees, a: DraftTrees): ChangeGroup[] {
  const groups: ChangeGroup[] = []
  const was = choiceListNames(b.schema)
  const now = choiceListNames(a.schema)
  for (const name of now) {
    const to = configureRoute.choiceList(name)
    const title = choiceListLabel(name)
    const before = getIn(b.schema, ['types', name])
    const after = getIn(a.schema, ['types', name])
    if (!was.includes(name)) {
      groups.push({
        key: `choices:${name}`,
        title,
        items: [
          {
            kind: 'added',
            label: `Choice list ${title}`,
            detail: `${strList(get(after, 'values')).length} options`,
            to,
          },
        ],
      })
      continue
    }
    const items = optionChanges(
      before,
      after,
      getIn(b.dataEntry, ['styles', name]),
      getIn(a.dataEntry, ['styles', name]),
      to
    )
    if (items.length) groups.push({ key: `choices:${name}`, title, items })
  }
  for (const name of was) {
    if (!now.includes(name)) {
      groups.push({
        key: `choices:${name}`,
        title: choiceListLabel(name),
        items: [
          {
            kind: 'removed',
            label: `Choice list ${choiceListLabel(name)}`,
            to: configureRoute.choiceLists(),
          },
        ],
      })
    }
  }
  return groups
}

/** A named section of mappings: relations in the data model, forms or lists on screen. */
function namedChanges(
  before: TreeValue | undefined,
  after: TreeValue | undefined,
  what: string,
  title: (name: string, def: TreeValue | undefined) => string,
  route: (name: string) => string,
  indexRoute: string,
  inner?: (name: string, old: TreeValue, now: TreeValue, to: string) => ChangeItem[]
): ChangeGroup[] {
  const groups: ChangeGroup[] = []
  for (const [name, def] of entries(after)) {
    const to = route(name)
    const old = get(before, name)
    const t = title(name, def)
    if (old === undefined) {
      groups.push({
        key: `${what}:${name}`,
        title: t,
        items: [{ kind: 'added', label: `${what} ${t}`, to }],
      })
      continue
    }
    const items = inner ? inner(name, old, def, to) : settingChanges(old, def, to)
    if (items.length) groups.push({ key: `${what}:${name}`, title: t, items })
  }
  for (const [name, def] of entries(before)) {
    if (get(after, name) === undefined) {
      const t = title(name, def)
      groups.push({
        key: `${what}:${name}`,
        title: t,
        items: [{ kind: 'removed', label: `${what} ${t}`, to: indexRoute }],
      })
    }
  }
  return groups
}

/** Items of a list section (rules, automations, dashboard cards), matched by origin. */
function itemChanges(
  before: TreeValue | undefined,
  after: TreeValue | undefined,
  what: string,
  title: (item: TreeMap, index: number) => string,
  route: (index: number) => string,
  orderLabel: string,
  indexRoute: string
): ChangeItem[] {
  const diff = diffList(before, after)
  const now = isList(after) ? after : []
  const out: ChangeItem[] = []
  for (const item of diff.added) {
    const i = now.indexOf(item)
    out.push({ kind: 'added', label: `${what} ${title(item, i)}`, to: route(i) })
  }
  for (const [old, item] of diff.changed) {
    const i = now.indexOf(item)
    const changed = settingChanges(old, item, route(i)).map((c) => c.label)
    out.push({
      kind: 'changed',
      label: `${what} ${title(item, i)}`,
      detail: changed.length ? changed.join(', ') : undefined,
      to: route(i),
    })
  }
  for (const item of diff.removed) {
    out.push({ kind: 'removed', label: `${what} ${title(item, item.$i ?? 0)}`, to: indexRoute })
  }
  if (diff.reordered) out.push({ kind: 'changed', label: orderLabel, to: indexRoute })
  return out
}

// --- Screens ---

function fieldTitle(item: TreeMap): string {
  return (
    str(get(item, 'label')) || (str(get(item, 'property')) ?? str(get(item, 'relation')) ?? 'field')
  )
}

function formInner(_name: string, old: TreeValue, now: TreeValue, to: string): ChangeItem[] {
  const fields = itemChanges(
    get(old, 'fields'),
    get(now, 'fields'),
    'Field',
    fieldTitle,
    () => to,
    'Order of fields',
    to
  )
  const rels = itemChanges(
    get(old, 'relations'),
    get(now, 'relations'),
    'Linked records',
    fieldTitle,
    () => to,
    'Order of linked records',
    to
  )
  return [...settingChanges(old, now, to, ['fields', 'relations']), ...fields, ...rels]
}

function listInner(_name: string, old: TreeValue, now: TreeValue, to: string): ChangeItem[] {
  const columns = itemChanges(
    get(old, 'columns'),
    get(now, 'columns'),
    'Column',
    (item, i) =>
      columnTitle({
        index: i,
        property: str(get(item, 'property')) ?? '',
        relation: str(get(item, 'relation')) ?? '',
        label: str(get(item, 'label')) ?? '',
        sortable: false,
        link: '',
      }),
    () => to,
    'Order of columns',
    to
  )
  return [...settingChanges(old, now, to, ['columns']), ...columns]
}

function navChanges(b: DraftTrees, a: DraftTrees): ChangeGroup[] {
  const groups: ChangeGroup[] = []
  const entryTitle = (item: TreeMap) =>
    str(get(item, 'label')) || str(get(item, 'group')) || (keys(item)[0] ?? 'entry')
  const navItems = (
    before: TreeValue | undefined,
    after: TreeValue | undefined,
    to: string
  ): ChangeItem[] => {
    const out = itemChanges(before, after, 'Entry', entryTitle, () => to, 'Order of entries', to)
    // Entries inside groups that are themselves unchanged in place.
    for (const [old, now] of diffList(before, after).changed) {
      if (get(now, 'items') === undefined && get(old, 'items') === undefined) continue
      out.push(...navItems(get(old, 'items'), get(now, 'items'), to))
    }
    return out
  }
  const spacesBefore = get(b.dataEntry, 'spaces')
  const spacesAfter = get(a.dataEntry, 'spaces')
  if (!equal(spacesBefore, spacesAfter)) {
    const diff = diffList(spacesBefore, spacesAfter)
    for (const space of diff.added) {
      const to = configureRoute.navigation(str(get(space, 'id')))
      groups.push({
        key: `space:new:${str(get(space, 'id'))}`,
        title: `Navigation: ${entryTitle(space)}`,
        items: [{ kind: 'added', label: `Space ${entryTitle(space)}`, to }],
      })
    }
    for (const [old, now] of diff.changed) {
      const to = configureRoute.navigation(str(get(now, 'id')))
      const items = [
        ...settingChanges(old, now, to, ['navigation', 'home']),
        ...navItems(get(old, 'navigation'), get(now, 'navigation'), to),
      ]
      if (items.length)
        groups.push({ key: `space:${old.$i}`, title: `Navigation: ${entryTitle(now)}`, items })
    }
    for (const space of diff.removed) {
      groups.push({
        key: `space:${space.$i}`,
        title: `Navigation: ${entryTitle(space)}`,
        items: [
          { kind: 'removed', label: `Space ${entryTitle(space)}`, to: configureRoute.navigation() },
        ],
      })
    }
  }
  const navBefore = get(b.dataEntry, 'navigation')
  const navAfter = get(a.dataEntry, 'navigation')
  if (!equal(navBefore, navAfter)) {
    const to = configureRoute.navigation()
    const items = navItems(navBefore, navAfter, to)
    if (items.length) groups.push({ key: 'navigation', title: 'Navigation', items })
  }
  return groups
}

const HANDLED_SCHEMA = ['entities', 'types', 'relations', 'validations', 'automations']
const HANDLED_SCREENS = ['styles', 'forms', 'lists', 'kanbans', 'dashboard', 'spaces', 'navigation']

/** Every change in the draft, grouped by what it changes. */
export function describeChanges(
  b: DraftTrees,
  a: DraftTrees,
  renames: ConfigRename[]
): ChangeGroup[] {
  const groups: ChangeGroup[] = []
  if (!equal(b.schema, a.schema)) {
    groups.push(...entityTypeChanges(b, a, renames))
    groups.push(...choiceListChanges(b, a))
    groups.push(
      ...namedChanges(
        get(b.schema, 'relations'),
        get(a.schema, 'relations'),
        'relation',
        (name, def) => str(get(def, 'label')) || name,
        configureRoute.relation,
        configureRoute.relations()
      )
    )
    const rules = itemChanges(
      get(b.schema, 'validations'),
      get(a.schema, 'validations'),
      'Rule',
      (item, i) =>
        ruleTitle({
          name: str(get(item, 'name')) ?? '',
          description: str(get(item, 'description')) ?? '',
          index: i,
        }),
      configureRoute.rule,
      'Order of rules',
      configureRoute.rules()
    )
    if (rules.length) groups.push({ key: 'rules', title: 'Rules', items: rules })
    const autos = itemChanges(
      get(b.schema, 'automations'),
      get(a.schema, 'automations'),
      'Automation',
      (item, i) =>
        automationTitle({
          name: str(get(item, 'name')) ?? '',
          description: str(get(item, 'description')) ?? '',
          index: i,
        }),
      configureRoute.automation,
      'Order of automations',
      configureRoute.automations()
    )
    if (autos.length) groups.push({ key: 'automations', title: 'Automations', items: autos })
    const other = settingChanges(b.schema, a.schema, configureRoute.entityTypes(), HANDLED_SCHEMA)
    if (other.length) groups.push({ key: 'schema-other', title: 'Data model', items: other })
  }
  if (!equal(b.dataEntry, a.dataEntry)) {
    groups.push(
      ...namedChanges(
        get(b.dataEntry, 'forms'),
        get(a.dataEntry, 'forms'),
        'form',
        (name, def) => str(get(def, 'title')) || name,
        configureRoute.form,
        configureRoute.forms(),
        formInner
      ),
      ...namedChanges(
        get(b.dataEntry, 'lists'),
        get(a.dataEntry, 'lists'),
        'list',
        (name, def) => str(get(def, 'title')) || name,
        configureRoute.list,
        configureRoute.lists(),
        listInner
      ),
      ...namedChanges(
        get(b.dataEntry, 'kanbans'),
        get(a.dataEntry, 'kanbans'),
        'board',
        (name, def) => str(get(def, 'title')) || name,
        configureRoute.board,
        configureRoute.boards(),
        (_n, old, now, to) => [
          ...settingChanges(old, now, to, ['columns', 'card']),
          ...itemChanges(
            get(old, 'columns'),
            get(now, 'columns'),
            'Column',
            (item) => str(get(item, 'label')) || (str(get(item, 'value')) ?? ''),
            () => to,
            'Order of columns',
            to
          ),
          ...(equal(get(old, 'card'), get(now, 'card'))
            ? []
            : [{ kind: 'changed' as const, label: 'Cards', to }]),
        ]
      )
    )
    const dashBefore = get(b.dataEntry, 'dashboard')
    const dashAfter = get(a.dataEntry, 'dashboard')
    if (!equal(dashBefore, dashAfter)) {
      const to = configureRoute.dashboard()
      const items = [
        ...settingChanges(dashBefore, dashAfter, to, ['cards']),
        ...itemChanges(
          get(dashBefore, 'cards'),
          get(dashAfter, 'cards'),
          'Card',
          (item) => str(get(item, 'title')) ?? 'card',
          () => to,
          'Order of cards',
          to
        ),
      ]
      if (items.length) groups.push({ key: 'dashboard', title: 'Dashboard', items })
    }
    groups.push(...navChanges(b, a))
    // Colours of names that are not choice lists (a choice list's colours
    // are listed with its options).
    const lists = new Set([...choiceListNames(b.schema), ...choiceListNames(a.schema)])
    const styleItems = namedChanges(
      get(b.dataEntry, 'styles'),
      get(a.dataEntry, 'styles'),
      'colours',
      (name) => name,
      () => configureRoute.choiceLists(),
      configureRoute.choiceLists()
    ).filter((g) => !lists.has(g.key.slice('colours:'.length)))
    groups.push(...styleItems)
    const app = settingChanges(
      get(b.dataEntry, 'app'),
      get(a.dataEntry, 'app'),
      configureRoute.navigation()
    )
    if (app.length) groups.push({ key: 'app', title: 'App', items: app })
    const other = settingChanges(b.dataEntry, a.dataEntry, configureRoute.navigation(), [
      ...HANDLED_SCREENS,
      'app',
    ])
    if (other.length) groups.push({ key: 'screens-other', title: 'Screens', items: other })
  }
  return groups
}

export function changeCount(groups: ChangeGroup[]): number {
  return groups.reduce((n, g) => n + g.items.length, 0)
}
