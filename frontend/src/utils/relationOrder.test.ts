import { describe, expect, it } from 'vitest'
import type { RelationOrder, RelationType, SidebarPage } from '@/types'
import { orderMoveArgs, planRowMove, tabIsRelationOrdered } from './relationOrder'

const rows = ['a', 'b', 'c', 'd'].map((id) => ({ id }))
const ids = (r: { id: string }[]) => r.map((x) => x.id)

describe('planRowMove', () => {
  it.each([
    [
      'before an earlier row',
      { itemId: 'c', targetId: 'a', placement: 'before' as const },
      ['c', 'a', 'b', 'd'],
      { before: 'a' },
    ],
    [
      'after a later row',
      { itemId: 'a', targetId: 'c', placement: 'after' as const },
      ['b', 'c', 'a', 'd'],
      { after: 'c' },
    ],
    [
      'after the last row',
      { itemId: 'b', targetId: 'd', placement: 'after' as const },
      ['a', 'c', 'd', 'b'],
      { after: 'd' },
    ],
    ['a step up', { itemId: 'c', step: -1 as const }, ['a', 'c', 'b', 'd'], { before: 'b' }],
    ['a step down', { itemId: 'b', step: 1 as const }, ['a', 'c', 'b', 'd'], { after: 'c' }],
  ])('moves %s', (_name, move, want, position) => {
    const plan = planRowMove(rows, move)
    expect(plan && ids(plan.rows)).toEqual(want)
    expect(plan?.position).toEqual(position)
  })

  it('sends the step itself past the edge of the page', () => {
    expect(planRowMove(rows, { itemId: 'a', step: -1 })).toEqual({ rows, position: { step: -1 } })
    expect(planRowMove(rows, { itemId: 'd', step: 1 })).toEqual({ rows, position: { step: 1 } })
  })

  it.each([
    [
      'onto the place it already has (after the row above)',
      { itemId: 'c', targetId: 'b', placement: 'after' as const },
    ],
    [
      'onto the place it already has (before the row below)',
      { itemId: 'c', targetId: 'd', placement: 'before' as const },
    ],
    ['an unknown row', { itemId: 'x', targetId: 'a', placement: 'before' as const }],
    ['onto an unknown row', { itemId: 'a', targetId: 'x', placement: 'before' as const }],
  ])('plans nothing for a drop %s', (_name, move) => {
    expect(planRowMove(rows, move)).toBeNull()
  })
})

describe('tabIsRelationOrdered', () => {
  const page = (direction: 'outgoing' | 'incoming', count = 1) =>
    ({
      label: 'Project',
      tabs: [
        {
          id: 'tasks',
          label: 'Tasks',
          links: Array.from({ length: count }, () => ({
            type: 'task',
            relation: 'has-task',
            direction,
          })),
        },
      ],
    }) as unknown as SidebarPage
  const relations = (outgoing: boolean, incoming = false) =>
    new Map<string, RelationType>([
      ['has-task', { label: 'has task', from: [], to: [], orderable: { outgoing, incoming } }],
    ])

  it('is true for one outgoing link over an outgoing-orderable relation', () => {
    expect(tabIsRelationOrdered(page('outgoing'), 'tasks', relations(true))).toBe(true)
  })
  it('is true for one incoming link over an incoming-orderable relation', () => {
    expect(tabIsRelationOrdered(page('incoming'), 'tasks', relations(false, true))).toBe(true)
  })
  it.each([
    ['the relation is not orderable', page('outgoing'), relations(false)],
    [
      'the link is incoming and only the outgoing side is orderable',
      page('incoming'),
      relations(true),
    ],
    [
      'the link is outgoing and only the incoming side is orderable',
      page('outgoing'),
      relations(false, true),
    ],
    ['the tab has two links', page('outgoing', 2), relations(true)],
    ['there is no page', undefined, relations(true)],
  ])('is false when %s', (_name, p, rels) => {
    expect(tabIsRelationOrdered(p, 'tasks', rels)).toBe(false)
  })
})

describe('orderMoveArgs', () => {
  const outgoing: RelationOrder = {
    relation: 'has-task',
    anchor: 'PRJ-1',
    anchor_type: 'project',
    movable: true,
  }
  const incoming: RelationOrder = {
    relation: 'has-step',
    anchor: 'STP-1',
    anchor_type: 'step',
    movable: true,
    direction: 'incoming',
    addresses: { 'POL-1': 'POL-1@published' },
  }

  it('names outgoing rows by id and sends no direction', () => {
    expect(orderMoveArgs(outgoing, 'a', { before: 'b' })).toEqual([
      'project',
      'PRJ-1',
      'has-task',
      'a',
      { before: 'b' },
    ])
  })
  it.each([
    ['the moved row', 'POL-1', { step: 1 as const }, 'POL-1@published', { step: 1 }],
    [
      'the row it lands before',
      'REC-1',
      { before: 'POL-1' },
      'REC-1',
      { before: 'POL-1@published' },
    ],
    ['the row it lands after', 'REC-1', { after: 'POL-1' }, 'REC-1', { after: 'POL-1@published' }],
  ])(
    'names %s by its address on the incoming side',
    (_name, id, position, wantId, wantPosition) => {
      expect(orderMoveArgs(incoming, id, position)).toEqual([
        'step',
        'STP-1',
        'has-step',
        wantId,
        wantPosition,
        'incoming',
      ])
    }
  )
})
