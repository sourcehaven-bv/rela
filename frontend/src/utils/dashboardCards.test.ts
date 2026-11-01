import { describe, expect, it } from 'vitest'

import type { DashboardCardEntry, Entity, EntityType } from '@/types'
import {
  groupKey,
  listBreakdown,
  queryBreakdown,
  tableRows,
  type CardSchema,
} from './dashboardCards'

const types: Record<string, EntityType> = {
  deal: {
    label: 'Deal',
    properties: {
      stage: { type: 'enum', values: ['lead', 'won'], labels: { lead: 'Lead', won: 'Won' } },
      sector: { type: 'string' },
      title: { type: 'string' },
    },
  } as unknown as EntityType,
  lead: {
    label: 'Lead',
    properties: { stage: { type: 'enum', values: ['won', 'lead'] } },
  } as unknown as EntityType,
}

const schema: CardSchema = {
  getEntityType: (name) => types[name],
  getEnumLabel: (value, property, type) =>
    (types[type as string]?.properties[property ?? '']?.labels ?? {})[value],
  stylesForProperty: () => ({ won: 'badge-green' }),
  getList: (id) => (id === 'deals' ? { entity: 'deal' } : undefined),
}

function row(id: string, properties: Record<string, unknown>, extra: Partial<Entity> = {}): Entity {
  const entity: Entity = { id, type: 'deal', properties, ...extra }
  return entity
}

describe('groupKey', () => {
  it.each([
    ['open', 'open'],
    [3, '3'],
    [false, 'false'],
    [['a', 'b'], ''],
    [{ a: 1 }, ''],
    [null, ''],
    [Number.NaN, ''],
  ])('reads %j as %j', (value, key) => {
    expect(groupKey(value)).toBe(key)
  })
})

describe('queryBreakdown', () => {
  const keys = (r: { items: { key: string }[] }) => r.items.map((i) => i.key)

  it('orders declared values first, then others by count and value, unset last', () => {
    const result = queryBreakdown(
      schema,
      'stage',
      [
        row('1', {}),
        row('2', { stage: 'zeta' }),
        row('3', { stage: 'alpha' }),
        row('4', { stage: 'won' }),
        row('5', { stage: 'won' }),
        row('6', { stage: 'lead' }),
        row('7', { stage: 'beta' }),
        row('8', { stage: 'beta' }),
      ],
      'Unknown'
    )
    expect(keys(result)).toEqual(['lead', 'won', 'beta', 'alpha', 'zeta', ''])
    expect(result.items.map((i) => i.label)).toEqual([
      'Lead',
      'Won',
      'beta',
      'alpha',
      'zeta',
      'Unknown',
    ])
    expect(result.items.map((i) => i.badge)).toEqual([true, true, true, true, true, false])
    expect(result.partial).toBe(false)
  })

  it('leaves out a row whose value is hidden and reports it partial', () => {
    const result = queryBreakdown(
      schema,
      'stage',
      [row('1', { stage: 'won' }), row('2', {}, { _redacted: ['stage'] })],
      'Unknown'
    )
    expect(keys(result)).toEqual(['won'])
    expect(result.partial).toBe(true)
  })

  it('orders by count when rows of several types declare different enums', () => {
    const result = queryBreakdown(
      schema,
      'stage',
      [
        row('1', { stage: 'lead' }),
        row('2', { stage: 'won' }, { type: 'lead' }),
        row('3', { stage: 'won' }),
      ],
      'Unknown'
    )
    expect(keys(result)).toEqual(['won', 'lead'])
  })

  it('counts a list value as no value, as the server does', () => {
    const result = queryBreakdown(
      schema,
      'stage',
      [row('1', { stage: ['lead', 'won'] })],
      'Unknown'
    )
    expect(keys(result)).toEqual([''])
  })

  it('gives a non-enum property no badge', () => {
    const result = queryBreakdown(schema, 'sector', [row('1', { sector: 'zorg' })], 'Unknown')
    expect(result.items[0].badge).toBe(false)
  })
})

describe('listBreakdown', () => {
  const card = (group_by: string): DashboardCardEntry => ({
    title: 'T',
    list: 'deals',
    display: 'breakdown',
    group_by,
    index: 0,
  })

  it('badges and colours an enum from the list type, keeping the server order', () => {
    const items = listBreakdown(
      schema,
      card('stage'),
      {
        title: 'T',
        display: 'breakdown',
        list: 'deals',
        total: 2,
        groups: [
          { value: 'won', label: 'Won', count: 1, matched: 0 },
          { value: '', label: '', count: 1, matched: 0 },
        ],
      },
      'Unknown'
    )
    expect(items.map((i) => [i.key, i.label, i.badge])).toEqual([
      ['won', 'Won', true],
      ['', 'Unknown', false],
    ])
    expect(items[0].color).toBe('var(--rl-color-status-green)')
  })

  it('gives a non-enum group no badge', () => {
    const items = listBreakdown(
      schema,
      card('sector'),
      {
        title: 'T',
        display: 'breakdown',
        list: 'deals',
        total: 1,
        groups: [{ value: 'zorg', label: 'zorg', count: 1, matched: 0 }],
      },
      'Unknown'
    )
    expect(items[0].badge).toBe(false)
  })
})

describe('tableRows', () => {
  it('marks a hidden value, also in a link column', () => {
    const card: DashboardCardEntry = {
      title: 'T',
      display: 'table',
      index: 0,
      columns: [{ property: 'title', link: 'detail' }, { property: 'stage' }],
    }
    const [r] = tableRows(schema, card, [row('D-1', {}, { _redacted: ['title', 'stage'] })], 'UTC')
    expect(r.cells.map((c) => [c.hidden, c.link])).toEqual([
      [true, undefined],
      [true, undefined],
    ])
  })

  it('gives a widget cell no text and a link cell its formatted text', () => {
    const card: DashboardCardEntry = {
      title: 'T',
      display: 'table',
      index: 0,
      columns: [{ property: 'title', link: 'detail' }, { property: 'stage' }],
    }
    const [r] = tableRows(schema, card, [row('D-1', { title: 'Big', stage: 'won' })], 'UTC')
    expect(r.cells[0]).toMatchObject({ link: '/entity/deal/D-1', text: 'Big' })
    expect(r.cells[1].widget?.modelValue).toBe('won')
    expect(r.cells[1].text).toBe('')
  })
})
