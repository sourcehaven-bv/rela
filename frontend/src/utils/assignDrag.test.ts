import { describe, expect, it } from 'vitest'
import { assignTargetForHref } from './assignDrag'

const lookup = {
  listAssignType: (id: string) => (id === 'today' ? 'todo' : undefined),
  pageAssignType: (id: string) => (id === 'project' ? 'todo' : undefined),
}

describe('assignTargetForHref', () => {
  it.each([
    ['/list/today', { src: { list: 'today' }, entityType: 'todo' }],
    ['/s/work/list/today?x=1', { src: { list: 'today' }, entityType: 'todo' }],
    ['/p/project/PRJ-1', { src: { page: 'project', target: 'PRJ-1' }, entityType: 'todo' }],
    ['/list/other', null],
    ['/p/area/AR-1', null],
    ['/views/x', null],
  ])('%s', (href, want) => {
    expect(assignTargetForHref(href, lookup)).toEqual(want)
  })
})
