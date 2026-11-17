import { describe, expect, it } from 'vitest'
import {
  automations,
  automationsUsing,
  automationTitle,
  boardModel,
  canSortColumn,
  columnMoveAllowed,
  columnRemoveAllowed,
  columnSource,
  columnTitle,
  choiceListModel,
  dashboardModel,
  dashboardPath,
  dashboards,
  entityTypeLabel,
  entityTypeModel,
  formModel,
  listModel,
  propertiesOf,
  relationModel,
  relationSentence,
  rules,
  spaces,
} from '../models'
import { isEditable } from '../paths'
import { dataEntryFixture, flowsFixture, schemaFixture, snapshot, toTree } from './fixture'

const schema = toTree(schemaFixture)
const dataEntry = toTree(dataEntryFixture)
const flows = toTree(flowsFixture)

describe('screen models', () => {
  it('reads an entity type with its authored labels', () => {
    const ticket = entityTypeModel(schema, 'ticket')
    expect(ticket).toMatchObject({ label: 'Ticket', plural: 'Tickets', idPrefix: 'TKT-' })
    // A type without a label shows its identifier as written.
    expect(entityTypeLabel(toTree({ entities: { bug_report: {} } }), 'bug_report')).toBe(
      'bug_report'
    )
  })

  it('marks computed properties as managed and links choice lists', () => {
    const props = propertiesOf(schema, 'ticket')
    expect(props.map((p) => p.name)).toEqual(['title', 'status', 'effort', 'checksum'])
    expect(props.find((p) => p.name === 'checksum')?.managed).toBe(true)
    expect(props.find((p) => p.name === 'status')?.choiceList).toBe('status')
    expect(props.find((p) => p.name === 'title')?.managed).toBe(false)
  })

  it('reads a choice list with its colours and users', () => {
    const status = choiceListModel(schema, dataEntry, 'status')
    expect(status?.options.map((o) => [o.value, o.color])).toEqual([
      ['open', 'blue'],
      ['doing', undefined],
      ['done', 'green'],
    ])
    expect(status?.default).toBe('open')
    expect(status?.usedBy).toEqual([{ entityType: 'ticket', property: 'status' }])
  })

  it('reads a relation as a sentence both ways', () => {
    const r = relationModel(schema, 'implements')
    expect(r).toBeDefined()
    expect(r?.inverse).toBe('implementedBy')
    expect(relationSentence(schema, r!)).toContain('implements')
    expect(relationSentence(schema, r!, true)).toContain('implementedBy')
  })

  it('locks rules that run scripts', () => {
    expect(rules(schema).map((r) => r.usesScript)).toEqual([false, true])
  })

  it('reads the flows.yaml triggers in firing order, locking scripted ones', () => {
    const autos = automations(flows)
    expect(autos.map((a) => a.name)).toEqual(['stamp_done', 'copy_effort', 'scripted', 'spawn'])
    expect(autos.map((a) => a.readOnly)).toEqual([false, false, true, false])
    expect(autos[0]).toMatchObject({
      entityTypes: ['ticket'],
      trigger: 'status becomes done',
      outputs: [{ kind: 'set', summary: 'Set effort to 0', locked: false }],
    })
    expect(automationTitle(autos[0]!)).toBe('Clear the effort of done tickets')
    expect(automationTitle(autos[1]!)).toBe('copy_effort')
    expect(autos[2]!.outputs[0]).toMatchObject({ kind: 'script', locked: true })
    // A template, `on.deleted` or `on.faces` is not edited here either.
    const other = automations(
      toTree({
        triggers: {
          t: {
            on: { entity: 'ticket', created: true },
            outputs: [{ create: { type: 'ticket', template: 'x.md' } }],
          },
          d: {
            on: { entity: 'ticket', deleted: true },
            outputs: [{ set: { properties: { a: 'b' } } }],
          },
          f: { on: { entity: 'ticket', created: true, faces: ['en'] }, outputs: [] },
          // `relate` and `create.if_exists` are locked too (#2423).
          r: {
            on: { entity: 'ticket', created: true },
            outputs: [{ relate: { type: 'implements', to: 'FEAT-1' } }],
          },
          i: {
            on: { entity: 'ticket', created: true },
            outputs: [{ create: { type: 'ticket', if_exists: 'replace' } }],
          },
        },
      })
    )
    expect(other.map((a) => a.readOnly)).toEqual([true, true, true, true, true])
    expect(other[3]!.outputs[0]).toMatchObject({ kind: 'relate', locked: true })
    expect(other[4]!.outputs[0]).toMatchObject({ kind: 'create', locked: true })
    expect(automations(toTree({}))).toEqual([])
  })

  it('finds the automations a removal would break', () => {
    expect(automationsUsing(flows, 'ticket').map((a) => a.name)).toEqual([
      'stamp_done',
      'copy_effort',
      'scripted',
    ])
    expect(automationsUsing(flows, 'ticket', 'status').map((a) => a.name)).toEqual(['stamp_done'])
    expect(automationsUsing(flows, 'ticket', 'effort').map((a) => a.name)).toEqual([
      'stamp_done',
      'copy_effort',
    ])
    expect(automationsUsing(flows, 'ticket', 'effortless').map((a) => a.name)).toEqual([
      'copy_effort',
    ])
    expect(automationsUsing(flows, 'ticket', 'eff')).toEqual([])
    expect(automationsUsing(flows, 'feature').map((a) => a.name)).toEqual(['spawn'])
  })

  it('reads forms, lists, boards and the dashboard', () => {
    expect(formModel(dataEntry, 'new_ticket')?.fields.map((f) => [f.property, f.span])).toEqual([
      ['title', undefined],
      ['status', 6],
      ['effort', 6],
    ])
    expect(listModel(dataEntry, 'tickets')?.sort).toEqual([
      { index: 0, property: 'status', direction: 'asc' },
    ])
    expect(listModel(dataEntry, 'tickets')?.columns.map((c) => c.sortable)).toEqual([false, true])
    expect(boardModel(dataEntry, 'board')).toMatchObject({
      columnProperty: 'status',
      cardFields: ['effort'],
    })
    expect(dashboardModel(dataEntry).cards.map((c) => c.restricted)).toEqual([false, true])
  })

  // A `list:` card (TKT-2SGPJZ) is shown but not edited in Configure.
  it('marks a list card', () => {
    const cards = dashboardModel(
      toTree({
        dashboard: {
          cards: [
            { title: 'Q', query: 'type:ticket' },
            { title: 'L', list: 'open', display: 'matrix' },
          ],
        },
      })
    ).cards
    expect(cards.map((c) => [c.list, c.display])).toEqual([
      ['', 'count'],
      ['open', 'matrix'],
    ])
  })

  // Named dashboards (TKT-5LQNLY) sit under `dashboards.<name>`.
  it('reads named dashboards beside the default one', () => {
    const withNamed = toTree({
      ...dataEntryFixture,
      dashboards: {
        ops: { title: 'Operations', cards: [] },
        crm: {
          title: 'Pipeline',
          cards: [{ title: 'Open deals', query: 'type:deal', display: 'count' }],
        },
      },
    })
    expect(dashboards(withNamed)).toEqual([
      { name: '', title: 'Default' },
      { name: 'crm', title: 'Pipeline' },
      { name: 'ops', title: 'Operations' },
    ])
    expect(dashboardModel(withNamed, 'crm')).toMatchObject({
      title: 'Pipeline',
      cards: [{ title: 'Open deals', query: 'type:deal' }],
    })
    expect(dashboardModel(withNamed).cards.map((c) => c.title)).toEqual(['Open', 'Secret'])
    expect(dashboardPath()).toEqual(['dashboard'])
    expect(dashboardPath('crm')).toEqual(['dashboards', 'crm'])
  })

  it('groups navigation and locks gated entries', () => {
    const [only] = spaces(dataEntry)
    expect(only.index).toBe(-1)
    expect(only.label).toBe('Tracker')
    expect(only.groups.map((g) => [g.label, g.entries.length])).toEqual([
      ['Work', 2],
      ['', 1],
    ])
    expect(only.groups[0].entries[1]).toMatchObject({
      kind: 'Board',
      target: 'board',
      path: ['navigation', 0, 'items', 1],
    })
    expect(only.groups[1].entries[0].locked).toBe(true)
  })
})

// A busy column (TKT-TU3XH4): the page shows and keeps one that exists.
describe('busy columns', () => {
  const busyList = toTree({
    lists: {
      t: {
        entity_type: 'ticket',
        columns: [
          { property: 'title' },
          { busy: 'run' },
          { busy: 'run', label: 'Agent' },
          { face: true },
          { relation: 'blocks' },
        ],
      },
    },
  })
  const cols = listModel(busyList, 't')!.columns

  it('busy column model and title', () => {
    expect(cols[1]).toMatchObject({ busy: 'run', face: false, property: '', relation: '' })
    expect(cols[3]).toMatchObject({ busy: '', face: true })
    expect(columnTitle(cols[1])).toBe('run')
    expect(columnTitle(cols[2])).toBe('Agent')
    expect(columnTitle(cols[3])).toBe('face')
  })

  it('column source text', () => {
    expect(cols.map(columnSource)).toEqual(['title', 'busy: run', 'busy: run', 'face', 'blocks'])
    expect(cols.map(canSortColumn)).toEqual([true, false, false, false, false])
  })

  it('busy column cannot become first', () => {
    const rows = listModel(
      toTree({
        lists: { t: { columns: [{ property: 'title' }, { busy: 'run' }, { property: 'status' }] } },
      }),
      't'
    )!.columns
    expect(columnMoveAllowed(rows, 1, 0)).toBe(false)
    expect(columnMoveAllowed(rows, 0, 2)).toBe(false)
    expect(columnMoveAllowed(rows, 1, 2)).toBe(true)
    expect(columnMoveAllowed(rows, 2, 0)).toBe(true)
    expect(columnRemoveAllowed(rows, 0)).toBe(false)
    expect(columnRemoveAllowed(rows, 1)).toBe(true)
    expect(columnRemoveAllowed(rows, 2)).toBe(true)
  })
})

describe('isEditable', () => {
  const { editable } = snapshot()

  it.each([
    ['schema.yaml', ['entities', 'ticket', 'label'], true],
    ['schema.yaml', ['entities', 'ticket', 'id_type'], false],
    ['schema.yaml', ['types', 'status', 'values', 2], true],
    ['schema.yaml', ['relations', 'implements', 'inverse'], false],
    ['data-entry.yaml', ['navigation', 0, 'items', 1, 'label'], true],
    ['data-entry.yaml', ['navigation', 0, 'permission'], false],
    ['data-entry.yaml', ['forms', 'f', 'fields', 0, 'span'], true],
    ['data-entry.yaml', ['spaces', 0, 'label'], true],
  ] as const)('%s %j is %s', (file, path, expected) => {
    expect(isEditable(editable, file, [...path])).toBe(expected)
  })
})
