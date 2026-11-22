import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, RouterLinkStub, enableAutoUnmount } from '@vue/test-utils'

import { routeLocationKey } from 'vue-router'
import FlowRunsPanel from './FlowRunsPanel.vue'
import { ApiError } from '@/api/errors'
import type { Flow, FlowRun } from '@/api/flows'
import { resetFlowList } from '@/composables/useFlowList'
import { useSchemaStore } from '@/stores/schema'

const listFlowsMock = vi.fn<() => Promise<Flow[]>>()
const listSubjectRunsMock =
  vi.fn<(subject: string, signal?: AbortSignal, space?: string) => Promise<FlowRun[]>>()
// The flows the server says the reader may start; the mock adds them to the runs.
let mayStart: string[] = ['onboarding']
const startFlowRunMock = vi.fn<(flow: string, subject: string) => Promise<string>>()
const getFlowRunMock = vi.fn()
vi.mock('@/api/flows', () => ({
  listFlows: () => listFlowsMock(),
  listSubjectRuns: async (s: string, sig?: AbortSignal, space?: string) => ({
    runs: await listSubjectRunsMock(s, sig, space),
    startable: mayStart,
  }),
  startFlowRun: (f: string, s: string) => startFlowRunMock(f, s),
  getFlowRun: (...a: unknown[]) => getFlowRunMock(...a),
  completeFlowStep: vi.fn(),
  cancelFlowRun: vi.fn(),
  retryFlowRun: vi.fn(),
}))
vi.mock('@/api/entities', () => ({
  getEntity: vi.fn().mockResolvedValue({ properties: {} }),
}))
vi.mock('@/composables/useEvents', () => ({
  useEvents: () => ({ on: vi.fn(), off: vi.fn() }),
}))
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: vi.fn() }),
}))
vi.mock('@/utils/markdown', () => ({ renderMermaidDiagrams: vi.fn() }))

const onboarding: Flow = {
  name: 'onboarding',
  label: 'Onboarding',
  subject: 'person',
  version: 'v1',
  manual: true,
  per_subject: 'one',
  diagram: '',
}

function run(over: Partial<FlowRun> = {}): FlowRun {
  return {
    id: 'R1',
    flow: 'onboarding',
    flow_label: 'Onboarding',
    version: 'v1',
    subject: 'PER-1',
    state: 'collect',
    state_label: 'Collect documents',
    status: 'input_required',
    generation: 1,
    active: true,
    waiting: { roles: ['hr'], you_hold: true, writes: [] },
    log: [{ at: '2026-01-01T10:00:00Z', state: 'collect', via: 'start' }],
    _actions: { complete: false, manage: true, cancel: false, retry: false },
    ...over,
  }
}

function mountPanel(entityType = 'person') {
  return mount(FlowRunsPanel, {
    props: { entityType, entityId: 'PER-1' },
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
}

enableAutoUnmount(afterEach)

describe('FlowRunsPanel', () => {
  beforeEach(() => {
    resetFlowList()
    // A loaded schema of a project with flows (capabilities.flows).
    Object.assign(useSchemaStore(), { loaded: true, flowsEnabled: true })
    listFlowsMock.mockReset()
    listSubjectRunsMock.mockReset()
    startFlowRunMock.mockReset()
    getFlowRunMock.mockReset()
    mayStart = ['onboarding']
  })

  it('renders nothing and reads no runs when the project has no flows', async () => {
    listFlowsMock.mockRejectedValue(
      new ApiError('HTTP 404', { kind: 'http', status: 404, original: null })
    )
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-runs-panel"]').exists()).toBe(false)
    expect(listSubjectRunsMock).not.toHaveBeenCalled()
  })

  it('renders nothing when no flow runs on this type', async () => {
    listFlowsMock.mockResolvedValue([onboarding])
    const wrapper = mountPanel('task')
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-runs-panel"]').exists()).toBe(false)
    expect(listSubjectRunsMock).not.toHaveBeenCalled()
  })

  it('drops the runs when the read is refused', async () => {
    listFlowsMock.mockResolvedValue([{ ...onboarding, manual: false }])
    listSubjectRunsMock.mockRejectedValue(
      new ApiError('HTTP 404', { kind: 'http', status: 404, original: null })
    )
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-runs-panel"]').exists()).toBe(false)
  })

  it('names the space the page is open in, so the server can hide the panel there', async () => {
    listFlowsMock.mockResolvedValue([onboarding])
    listSubjectRunsMock.mockResolvedValue([])
    mount(FlowRunsPanel, {
      props: { entityType: 'person', entityId: 'PER-1' },
      global: {
        stubs: { RouterLink: RouterLinkStub },
        provide: { [routeLocationKey as symbol]: { path: '/s/portal/entity/person/PER-1', query: {} } },
      },
    })
    await flushPromises()
    expect(listSubjectRunsMock).toHaveBeenCalledWith('PER-1', expect.anything(), 'portal')
  })

  it('shows each run on the entity with why it waits', async () => {
    listFlowsMock.mockResolvedValue([onboarding])
    listSubjectRunsMock.mockResolvedValue([run()])
    const wrapper = mountPanel()
    await flushPromises()

    expect(listSubjectRunsMock).toHaveBeenCalledWith('PER-1', expect.any(AbortSignal), undefined)
    const cards = wrapper.findAll('[data-testid="flow-run"]')
    expect(cards).toHaveLength(1)
    expect(wrapper.find('[data-testid="flow-run-state"]').text()).toBe('Collect documents')
    expect(wrapper.find('[data-testid="flow-waiting-roles"]').text()).toContain('hr')
    // The subject's list carries the detail, so the card reads nothing more.
    expect(getFlowRunMock).not.toHaveBeenCalled()
  })

  it('starts a manual flow on the entity and reads the runs again', async () => {
    listFlowsMock.mockResolvedValue([onboarding])
    listSubjectRunsMock.mockResolvedValue([])
    startFlowRunMock.mockResolvedValue('R9')
    const wrapper = mountPanel()
    await flushPromises()

    const start = wrapper.find('[data-testid="flow-start-onboarding"]')
    expect(start.text()).toContain('Onboarding')
    await start.trigger('click')
    await flushPromises()

    expect(startFlowRunMock).toHaveBeenCalledWith('onboarding', 'PER-1')
    expect(listSubjectRunsMock).toHaveBeenCalledTimes(2)
  })

  it('offers no Start for a per_subject: one flow while a run of it is unfinished', async () => {
    const cases: [string, Flow, FlowRun[], boolean][] = [
      ['unfinished run, one', onboarding, [run()], false],
      ['finished run, one', onboarding, [run({ active: false, status: 'completed' })], true],
      ['unfinished run, many', { ...onboarding, per_subject: 'many' }, [run()], true],
      ['unfinished run of another flow', onboarding, [run({ flow: 'other' })], true],
    ]
    for (const [name, flow, runs, want] of cases) {
      resetFlowList()
      listFlowsMock.mockResolvedValue([flow])
      listSubjectRunsMock.mockResolvedValue(runs)
      const wrapper = mountPanel()
      await flushPromises()
      expect(wrapper.find('[data-testid="flow-start-onboarding"]').exists(), name).toBe(want)
      wrapper.unmount()
    }
  })

  it('offers no Start before the runs have loaded', async () => {
    listFlowsMock.mockResolvedValue([onboarding])
    let resolve: (r: FlowRun[]) => void = () => {}
    listSubjectRunsMock.mockReturnValue(new Promise((r) => (resolve = r)))
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-start-onboarding"]').exists()).toBe(false)
    resolve([])
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-start-onboarding"]').exists()).toBe(true)
  })

  it('offers no Start when the runs fail to load', async () => {
    listFlowsMock.mockResolvedValue([onboarding])
    listSubjectRunsMock.mockRejectedValue(new Error('boom'))
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-runs-panel"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="flow-start-onboarding"]').exists()).toBe(false)
  })

  it('offers Start only for a flow the server says the reader may start', async () => {
    listFlowsMock.mockResolvedValue([onboarding])
    listSubjectRunsMock.mockResolvedValue([run({ active: false, status: 'completed' })])
    mayStart = []
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-runs-panel"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="flow-start-onboarding"]').exists()).toBe(false)
  })

  // BUG-UZGMHIK: an action on the page bumps `revision`; the runs are read again.
  it('reads the runs again when the revision changes', async () => {
    listFlowsMock.mockResolvedValue([onboarding])
    listSubjectRunsMock.mockResolvedValue([run()])
    const wrapper = mountPanel()
    await flushPromises()
    const reads = listSubjectRunsMock.mock.calls.length
    await wrapper.setProps({ revision: 1 })
    await flushPromises()
    expect(listSubjectRunsMock).toHaveBeenCalledTimes(reads + 1)
  })
})
