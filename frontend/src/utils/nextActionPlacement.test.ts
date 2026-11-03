import { describe, it, expect } from 'vitest'
import router from '@/router'
import { nextActionPlacement, placementKey } from './nextActionPlacement'

/**
 * Resolved against the real router, so a renamed route or param fails here
 * rather than silently sending every page as `other`.
 */
describe('nextActionPlacement', () => {
  const cases: [string, ReturnType<typeof nextActionPlacement>][] = [
    ['/dashboard', { surface: 'dashboard' }],
    ['/dashboard/ops', { surface: 'dashboard', view: 'ops' }],
    ['/list/open-tasks', { surface: 'list', view: 'open-tasks' }],
    ['/kanban/board', { surface: 'kanban', view: 'board' }],
    ['/calendar/due', { surface: 'calendar', view: 'due' }],
    ['/gantt/plan', { surface: 'gantt', view: 'plan' }],
    ['/entity/task/TASK-1', { surface: 'entity', view: 'task', entity: 'TASK-1' }],
    ['/form/new-task', { surface: 'form', view: 'new-task' }],
    ['/form/edit-task/TASK-1', { surface: 'form', view: 'edit-task', entity: 'TASK-1' }],
    ['/document/report/TASK-1', { surface: 'document', view: 'report', entity: 'TASK-1' }],
    ['/document/overview', { surface: 'document', view: 'overview' }],
    ['/p/home', { surface: 'page', view: 'home' }],
    ['/p/task-page/TASK-1/details', { surface: 'page', view: 'task-page', entity: 'TASK-1' }],
    ['/search', { surface: 'search' }],
    ['/settings', { surface: 'other' }],
    ['/analyze', { surface: 'other' }],
    ['/conflicts', { surface: 'other' }],
    ['/history/task/TASK-1', { surface: 'other' }],
    ['/app/tool', { surface: 'other' }],
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
    expect(nextActionPlacement({ name: undefined, params: {} })).toEqual({ surface: 'other' })
  })

  it('keys placements by every field', () => {
    const a = placementKey({ surface: 'entity', view: 'task', entity: 'T-1' })
    expect(placementKey({ surface: 'entity', view: 'task', entity: 'T-2' })).not.toBe(a)
    expect(placementKey({ surface: 'entity', view: 'task', entity: 'T-1' })).toBe(a)
  })
})
