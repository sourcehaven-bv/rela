import { describe, expect, it } from 'vitest'
import {
  automations,
  boardModel,
  choiceListModel,
  dashboardModel,
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
import { dataEntryFixture, schemaFixture, snapshot, toTree } from './fixture'

const schema = toTree(schemaFixture)
const dataEntry = toTree(dataEntryFixture)

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

  it('locks rules and automations that run scripts', () => {
    expect(rules(schema).map((r) => r.usesScript)).toEqual([false, true])
    expect(automations(schema).map((a) => a.readOnly)).toEqual([false, true])
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
