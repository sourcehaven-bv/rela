import { describe, expect, it } from 'vitest'
import {
  boardModel,
  chooseCardFields,
  defaultText,
  filterControlsOf,
  fixedFiltersOf,
  listModel,
  navEntryModel,
  navGroups,
  queryScopeNames,
  relationsOfType,
  typedDefault,
} from '../models'
import { entries, getIn, isList, isMap, type TreeValue } from '../tree'
import { toTree } from './fixture'

describe('list and board settings', () => {
  const dataEntry = toTree({
    lists: {
      all: {
        entity_type: 'ticket',
        sort: [{ property: 'priority' }, { property: 'status', direction: 'desc' }],
        filter_controls: [
          { property: 'status', widget: 'multi-select' },
          { relation: 'implements', direction: 'incoming', label: 'Feature' },
        ],
        filters: [{ property: 'effort', operator: '>=', value: 3 }],
        condition: "entity.status ~= 'done'",
        query_scope: 'open_work',
        detail_view: 'ticket_page',
      },
    },
    kanbans: { board: { entity_type: 'ticket', query_scope: 'open_work' } },
  })

  it('reads every sort key, not only the first', () => {
    expect(listModel(dataEntry, 'all')?.sort).toEqual([
      { index: 0, property: 'priority', direction: 'asc' },
      { index: 1, property: 'status', direction: 'desc' },
    ])
  })

  it('reads filter controls on properties and relations', () => {
    expect(listModel(dataEntry, 'all')?.filterControls).toEqual([
      { index: 0, property: 'status', relation: '', direction: 'outgoing', label: '' },
      { index: 1, property: '', relation: 'implements', direction: 'incoming', label: 'Feature' },
    ])
  })

  it('reads fixed filters, with a number value as text', () => {
    expect(fixedFiltersOf(getIn(dataEntry, ['lists', 'all', 'filters']))).toEqual([
      { index: 0, property: 'effort', operator: '>=', value: '3' },
    ])
  })

  it('reads the condition, query scope and detail view', () => {
    expect(listModel(dataEntry, 'all')).toMatchObject({
      condition: "entity.status ~= 'done'",
      queryScope: 'open_work',
      detailView: 'ticket_page',
    })
    expect(boardModel(dataEntry, 'board')?.queryScope).toBe('open_work')
  })

  it('reads nothing from a missing list', () => {
    expect(filterControlsOf(undefined)).toEqual([])
  })
})

/** A tree as plain values, without list origins. */
function plain(value: TreeValue): unknown {
  if (isList(value)) return value.map(plain)
  if (isMap(value)) return Object.fromEntries(entries(value).map(([k, v]) => [k, plain(v)]))
  return value
}

describe('card fields', () => {
  const fields = toTree([
    { property: 'status', label: 'State', show_label: true },
    { relation: 'implements', direction: 'outgoing' },
    { property: 'effort' },
  ])

  it('keeps the settings of a field that stays, and every relation field', () => {
    expect(plain(chooseCardFields(fields, ['status', 'priority']))).toEqual([
      { property: 'status', label: 'State', show_label: true },
      { relation: 'implements', direction: 'outgoing' },
      { property: 'priority' },
    ])
  })

  it('starts a list when the board had none', () => {
    expect(plain(chooseCardFields(undefined, ['title']))).toEqual([{ property: 'title' }])
  })
})

describe('property defaults', () => {
  it('saves a default typed for its property', () => {
    expect(typedDefault('3', 'integer')).toBe(3)
    expect(typedDefault('true', 'boolean')).toBe(true)
    expect(typedDefault('false', 'boolean')).toBe(false)
    expect(typedDefault('open', 'status')).toBe('open')
    expect(typedDefault('  ', 'string')).toBeUndefined()
  })

  it('keeps text that does not read as the type, for the server to report', () => {
    expect(typedDefault('lots', 'integer')).toBe('lots')
  })

  it('shows scalar defaults as text and has no text for a list', () => {
    expect(defaultText(5)).toBe('5')
    expect(defaultText(false)).toBe('false')
    expect(defaultText(undefined)).toBe('')
    const list = toTree(['a', 'b'])
    expect(isList(list)).toBe(true)
    expect(defaultText(list)).toBeUndefined()
  })
})

describe('schema lookups for screens', () => {
  const schema = toTree({
    entities: {
      ticket: { query_scopes: { open_work: "entity.status ~= 'done'", mine: 'x' } },
      feature: {},
    },
    relations: {
      implements: { from: ['ticket'], to: ['feature'], inverse: 'implemented by' },
      relates: {},
    },
  })

  it('lists the query scopes of a type', () => {
    expect(queryScopeNames(schema, 'ticket')).toEqual(['open_work', 'mine'])
    expect(queryScopeNames(schema, 'feature')).toEqual([])
  })

  it('lists relations by the side the type is on', () => {
    const named = (t: string) => relationsOfType(schema, t).map((r) => `${r.name}:${r.direction}`)
    expect(named('ticket')).toEqual(['implements:outgoing', 'relates:outgoing', 'relates:incoming'])
    expect(named('feature')).toEqual([
      'implements:incoming',
      'relates:outgoing',
      'relates:incoming',
    ])
    const incoming = relationsOfType(schema, 'feature').find((r) => r.name === 'implements')
    expect(incoming?.label).toBe('implemented by')
  })
})

describe('navigation groups', () => {
  it('lets a group filled from a list be edited, but not one behind a permission', () => {
    const nav = toTree([
      { group: 'Topics', items_from: { list: 'topics', limit: 5 } },
      { group: 'Admin', permission: 'admin', items: [{ list: 'users' }] },
    ])
    const groups = navGroups(nav, ['navigation'])
    expect(groups.map((g) => [g.label, g.fromList, g.locked])).toEqual([
      ['Topics', true, false],
      ['Admin', false, true],
    ])
  })
})

describe('navigation entry icons', () => {
  const iconOf = (item: Record<string, unknown>) =>
    navEntryModel(toTree(item) as never, 0, ['navigation', 0]).icon

  it('derives the glyph the server picks for each kind', () => {
    expect(iconOf({ gantt: 'plan' })).toBe('gantt')
    expect(iconOf({ entities: 'feature' })).toBe('file')
    expect(iconOf({ page: 'home' })).toBe('')
  })

  it('prefers the icon the entry names, including none', () => {
    expect(iconOf({ list: 'bugs', icon: 'bug' })).toBe('bug')
    expect(iconOf({ list: 'bugs', icon: 'none' })).toBe('none')
  })
})
