import { describe, it, expect } from 'vitest'
import { sortGanttNodes } from './ganttSort'
import type { GanttNode } from '@/api'

function node(id: string, title: string, start?: string, end?: string, children: GanttNode[] = []): GanttNode {
  return { id, type: 'project', title, planned: start || end ? { start, end } : undefined, children }
}

const ids = (nodes: GanttNode[]) => nodes.map((n) => n.id)

describe('sortGanttNodes', () => {
  const list = [
    node('B', 'bravo', '2026-03-01', '2026-04-01'),
    node('A', 'Alpha', '2026-05-01', '2026-06-01'),
    node('C', 'Charlie'),
    node('D', 'delta', '2026-01-01', '2026-02-01'),
  ]

  it('leaves the order alone without a sort', () => {
    expect(sortGanttNodes(list, [])).toBe(list)
  })

  it('sorts by title without regard to case', () => {
    expect(ids(sortGanttNodes(list, [{ property: 'title', direction: 'asc' }]))).toEqual(['A', 'B', 'C', 'D'])
    expect(ids(sortGanttNodes(list, [{ property: 'title', direction: 'desc' }]))).toEqual(['D', 'C', 'B', 'A'])
  })

  it('sorts by start date and puts undated rows last in both directions', () => {
    expect(ids(sortGanttNodes(list, [{ property: 'start', direction: 'asc' }]))).toEqual(['D', 'B', 'A', 'C'])
    expect(ids(sortGanttNodes(list, [{ property: 'start', direction: 'desc' }]))).toEqual(['A', 'B', 'D', 'C'])
  })

  it('sorts by end date', () => {
    expect(ids(sortGanttNodes(list, [{ property: 'end', direction: 'desc' }]))).toEqual(['A', 'B', 'D', 'C'])
  })

  it('reads the rolled span before the planned window', () => {
    const rolled: GanttNode = { ...node('R', 'rolled', '2026-12-01', '2026-12-31'), rolled: { start: '2026-01-01' } }
    const other = node('O', 'other', '2026-06-01')
    expect(ids(sortGanttNodes([other, rolled], [{ property: 'start', direction: 'asc' }]))).toEqual(['R', 'O'])
  })

  it('breaks a tie with the next key', () => {
    const tied = [node('X', 'zed', '2026-01-01'), node('Y', 'abe', '2026-01-01')]
    const sorted = sortGanttNodes(tied, [
      { property: 'start', direction: 'asc' },
      { property: 'title', direction: 'asc' },
    ])
    expect(ids(sorted)).toEqual(['Y', 'X'])
  })

  it('sorts the siblings of every level and does not change its input', () => {
    const tree = [node('P', 'parent', undefined, undefined, [node('2', 'two'), node('1', 'one')])]
    const sorted = sortGanttNodes(tree, [{ property: 'title', direction: 'asc' }])

    expect(ids(sorted[0].children!)).toEqual(['1', '2'])
    expect(ids(tree[0].children!)).toEqual(['2', '1'])
  })

  it('ignores a key it does not know', () => {
    expect(ids(sortGanttNodes(list, [{ property: 'ghost', direction: 'asc' }]))).toEqual(['B', 'A', 'C', 'D'])
  })
})
