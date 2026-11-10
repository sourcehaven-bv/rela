import { describe, it, expect, vi, beforeEach } from 'vitest'
import { loadFlowList, resetFlowList, useFlowList } from './useFlowList'
import type { Flow } from '@/api/flows'

const listFlowsMock = vi.fn<() => Promise<Flow[]>>()
vi.mock('@/api/flows', () => ({ listFlows: () => listFlowsMock() }))

const flow: Flow = {
  name: 'onboarding',
  label: 'Onboarding',
  subject: 'person',
  version: 'v1',
  manual: true,
  diagram: '',
}

describe('useFlowList', () => {
  beforeEach(() => {
    resetFlowList()
    listFlowsMock.mockReset()
  })

  it('reads the list once and shares it', async () => {
    listFlowsMock.mockResolvedValue([flow])
    const a = useFlowList()
    const b = useFlowList()
    await loadFlowList()
    expect(listFlowsMock).toHaveBeenCalledTimes(1)
    expect(a.hasFlows.value).toBe(true)
    expect(b.flowNamed('onboarding')?.subject).toBe('person')
    expect(b.flowNamed('other')).toBeUndefined()
    expect(a.loaded.value).toBe(true)
  })

  it('reads a failure as no flows and reads again on the next call', async () => {
    listFlowsMock.mockRejectedValueOnce(new Error('HTTP 404'))
    const list = useFlowList()
    await loadFlowList()
    expect(list.hasFlows.value).toBe(false)
    expect(list.loaded.value).toBe(false)

    listFlowsMock.mockResolvedValue([flow])
    await loadFlowList()
    expect(listFlowsMock).toHaveBeenCalledTimes(2)
    expect(list.hasFlows.value).toBe(true)
  })
})
