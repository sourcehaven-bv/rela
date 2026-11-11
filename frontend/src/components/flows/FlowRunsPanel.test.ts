import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, RouterLinkStub, enableAutoUnmount } from '@vue/test-utils'

import FlowRunsPanel from './FlowRunsPanel.vue'
import { ApiError } from '@/api/errors'
import type { Flow, FlowRun } from '@/api/flows'
import { resetFlowList } from '@/composables/useFlowList'

const listFlowsMock = vi.fn<() => Promise<Flow[]>>()
const listSubjectRunsMock = vi.fn<(subject: string, signal?: AbortSignal) => Promise<FlowRun[]>>()
const startFlowRunMock = vi.fn<(flow: string, subject: string) => Promise<string>>()
const getFlowRunMock = vi.fn()
vi.mock('@/api/flows', () => ({
  listFlows: () => listFlowsMock(),
  listSubjectRuns: (s: string, sig?: AbortSignal) => listSubjectRunsMock(s, sig),
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
    _actions: { complete: false, cancel: false, retry: false },
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
    listFlowsMock.mockReset()
    listSubjectRunsMock.mockReset()
    startFlowRunMock.mockReset()
    getFlowRunMock.mockReset()
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

  it('shows each run on the entity with why it waits', async () => {
    listFlowsMock.mockResolvedValue([onboarding])
    listSubjectRunsMock.mockResolvedValue([run()])
    const wrapper = mountPanel()
    await flushPromises()

    expect(listSubjectRunsMock).toHaveBeenCalledWith('PER-1', expect.any(AbortSignal))
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
})
