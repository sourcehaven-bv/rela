import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import type { Flow, FlowRun, FlowRunPage, FlowTask, FlowTaskPage } from '@/api/flows'
import { resetFlowList } from '@/composables/useFlowList'
import FlowRunsView from './FlowRunsView.vue'
import FlowTasksView from './FlowTasksView.vue'
import { useSchemaStore } from '@/stores/schema'

// The run and task lists name a subject by its title and fall back to its
// address; their links carry the app's link class.

const api = vi.hoisted(() => ({
  listFlows: vi.fn<() => Promise<Flow[]>>(),
  listFlowRuns: vi.fn<() => Promise<FlowRunPage>>(),
  listFlowTasks: vi.fn<() => Promise<FlowTaskPage>>(),
  retryFlowRun: vi.fn<(id: string) => Promise<void>>(),
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
  per_subject: 'many',
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
    // A loaded schema of a project with flows (capabilities.flows).
    Object.assign(useSchemaStore(), { loaded: true, flowsEnabled: true })
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

  it('lists failed runs with a Retry button above the tasks, and retries one', async () => {
    api.listFlowTasks.mockResolvedValue({
      tasks: [],
      failed: [task({ run: 'R9', state: 'provision', state_label: 'Provision' })],
      total: 0,
      truncated: false,
    })
    api.retryFlowRun.mockResolvedValue()
    const w = mount(FlowTasksView)
    await flushPromises()
    const rows = w.findAll('[data-testid="flow-failed-row"]')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('Provision')
    // A failed run is something to do, so the list is not empty.
    expect(w.text()).not.toContain('Nothing waits for you')

    await w.find('[data-testid="flow-failed-retry"]').trigger('click')
    await flushPromises()
    expect(api.retryFlowRun).toHaveBeenCalledWith('R9')
    expect(api.listFlowTasks).toHaveBeenCalledTimes(2)
  })

  it('shows no failed runs section when the server sends none', async () => {
    api.listFlowTasks.mockResolvedValue({ tasks: [], failed: [], total: 0, truncated: false })
    const w = mount(FlowTasksView)
    await flushPromises()
    expect(w.find('[data-testid="flow-tasks-failed"]').exists()).toBe(false)
    expect(w.text()).toContain('Nothing waits for you')
  })
})
