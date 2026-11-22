import { describe, it, expect } from 'vitest'
import router from '@/router'
import { leadsHere, nextActionPlacement, placementKey } from './nextActionPlacement'

/**
 * Resolved against the real router, so a renamed route or param fails here
 * rather than silently sending every page as `other`.
 */
describe('nextActionPlacement', () => {
  const cases: [string, ReturnType<typeof nextActionPlacement>][] = [
    ['/dashboard', { kind: 'dashboard' }],
    ['/dashboard/ops', { kind: 'dashboard', view: 'ops' }],
    ['/list/open-tasks', { kind: 'list', view: 'open-tasks' }],
    ['/kanban/board', { kind: 'kanban', view: 'board' }],
    ['/calendar/due', { kind: 'calendar', view: 'due' }],
    ['/gantt/plan', { kind: 'gantt', view: 'plan' }],
    ['/entity/task/TASK-1', { kind: 'entity', view: 'task', entity: 'TASK-1' }],
    ['/form/new-task', { kind: 'form', view: 'new-task' }],
    ['/form/edit-task/TASK-1', { kind: 'form', view: 'edit-task', entity: 'TASK-1' }],
    ['/document/report/TASK-1', { kind: 'document', view: 'report', entity: 'TASK-1' }],
    ['/document/overview', { kind: 'document', view: 'overview' }],
    ['/p/home', { kind: 'page', view: 'home' }],
    ['/p/task-page/TASK-1/details', { kind: 'page', view: 'task-page', entity: 'TASK-1' }],
    ['/search', { kind: 'search' }],
    ['/settings', { kind: 'other' }],
    ['/analyze', { kind: 'other' }],
    ['/conflicts', { kind: 'other' }],
    ['/history/task/TASK-1', { kind: 'other' }],
    ['/app/tool', { kind: 'other' }],
  ]

  for (const [path, want] of cases) {
    it(`maps ${path}`, () => {
      expect(nextActionPlacement(router.resolve(path))).toEqual(want)
    })
    it(`maps ${path} under a space prefix`, () => {
      expect(nextActionPlacement(router.resolve(`/s/crm${path}`))).toEqual(want)
    })
  }

  it('maps an unnamed route to other', () => {
    expect(nextActionPlacement({ name: undefined, params: {} })).toEqual({ kind: 'other' })
  })

  it('keys placements by every field', () => {
    const a = placementKey({ kind: 'entity', view: 'task', entity: 'T-1' })
    expect(placementKey({ kind: 'entity', view: 'task', entity: 'T-2' })).not.toBe(a)
    expect(placementKey({ kind: 'entity', view: 'task', entity: 'T-1' })).toBe(a)
  })
})

describe('leadsHere', () => {
  const here = (path: string) => router.resolve(path)

  it('matches the page a navigate offer leads to, query included', () => {
    const target = router.resolve('/entity/account/ACC-1?run=RUN-1')
    expect(leadsHere(target, here('/entity/account/ACC-1?run=RUN-1'))).toBe(true)
    expect(leadsHere(target, here('/entity/account/ACC-1?run=RUN-1&tab=x'))).toBe(true)
  })

  it('does not match another entity, another run or no run', () => {
    const target = router.resolve('/entity/account/ACC-1?run=RUN-1')
    expect(leadsHere(target, here('/entity/account/ACC-2?run=RUN-1'))).toBe(false)
    expect(leadsHere(target, here('/entity/account/ACC-1?run=RUN-2'))).toBe(false)
    expect(leadsHere(target, here('/entity/account/ACC-1'))).toBe(false)
    expect(leadsHere(target, here('/dashboard'))).toBe(false)
  })
})
