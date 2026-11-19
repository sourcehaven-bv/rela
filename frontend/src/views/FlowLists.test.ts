import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import type { Flow, FlowRun, FlowRunPage, FlowTask, FlowTaskPage } from '@/api/flows'
import { resetFlowList } from '@/composables/useFlowList'
import FlowRunsView from './FlowRunsView.vue'
import FlowTasksView from './FlowTasksView.vue'

// The run and task lists name a subject by its title and fall back to its
// address; their links carry the app's link class.

const api = vi.hoisted(() => ({
  listFlows: vi.fn<() => Promise<Flow[]>>(),
  listFlowRuns: vi.fn<() => Promise<FlowRunPage>>(),
  listFlowTasks: vi.fn<() => Promise<FlowTaskPage>>(),
}))
vi.mock('@/api/flows', async (orig) => ({
  ...(await orig<typeof import('@/api/flows')>()),
  ...api,
}))
vi.mock('@/composables/useEvents', () => ({ useEvents: () => ({ on: vi.fn() }) }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
  RouterLink: defineComponent({
    props: ['to'],
    template: '<a :href="to"><slot /></a>',
  }),
}))

const onboarding: Flow = {
  name: 'onboarding',
  label: 'Onboarding',
  subject: 'person',
  version: 'v1',
  manual: true,
  diagram: '',
}

function run(over: Partial<FlowRun>): FlowRun {
  return {
    id: 'R1',
    flow: 'onboarding',
    flow_label: 'Onboarding',
    version: 'v1',
    subject: 'PER-1',
    state: 'collect',
    state_label: 'Collect',
    status: 'input_required',
    generation: 1,
    active: true,
    _actions: { complete: false, cancel: false, retry: false },
    ...over,
  }
}

function task(over: Partial<FlowTask>): FlowTask {
  return {
    run: 'R1',
    flow: 'onboarding',
    flow_label: 'Onboarding',
    state: 'collect',
    state_label: 'Collect',
    subject: 'PER-1',
    ...over,
  }
}

enableAutoUnmount(afterEach)

describe('flow lists', () => {
  beforeEach(() => {
    resetFlowList()
    api.listFlows.mockResolvedValue([onboarding])
  })

  it('names a run subject by its title, else by its address', async () => {
    api.listFlowRuns.mockResolvedValue({
      runs: [run({ id: 'R1', subject_title: 'Ada' }), run({ id: 'R2', subject: 'PER-2' })],
      total: 2,
      truncated: false,
    })
    const w = mount(FlowRunsView)
    await flushPromises()
    const links = w.findAll('[data-testid="flow-run-subject"]')
    expect(links.map((l) => l.text())).toEqual(['Ada', 'PER-2'])
    for (const l of w.findAll('[data-testid="flow-run-row"] a')) {
      expect(l.classes()).toContain('flow-list__link')
    }
  })

  it('names a task subject by its title, else by its address', async () => {
    api.listFlowTasks.mockResolvedValue({
      tasks: [
        task({ run: 'R1', subject_title: 'Ada' }),
        task({ run: 'R2', subject: 'PER-2' }),
        // A flow the list does not know has no subject type, so no link.
        task({ run: 'R3', flow: 'gone', subject: 'PER-3', subject_title: 'Cy' }),
      ],
      total: 3,
      truncated: false,
    })
    const w = mount(FlowTasksView)
    await flushPromises()
    const cells = w.findAll('[data-testid="flow-task-subject"]')
    expect(cells.map((c) => c.text())).toEqual(['Ada', 'PER-2', 'Cy'])
    expect(cells[2].element.tagName).toBe('SPAN')
    for (const l of w.findAll('tbody a')) expect(l.classes()).toContain('flow-list__link')
  })
})
