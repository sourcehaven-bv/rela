import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { loadFlowList, resetFlowList, useFlowList } from './useFlowList'
import { useSchemaStore } from '@/stores/schema'
import type { Flow } from '@/api/flows'

const listFlowsMock = vi.fn<() => Promise<Flow[]>>()
vi.mock('@/api/flows', () => ({ listFlows: () => listFlowsMock() }))

const flow: Flow = {
  name: 'onboarding',
  label: 'Onboarding',
  subject: 'person',
  version: 'v1',
  manual: true,
  per_subject: 'many',
  diagram: '',
}

describe('useFlowList', () => {
  beforeEach(() => {
    // A loaded schema of a project with flows; load() is then a no-op.
    Object.assign(useSchemaStore(), { loaded: true, flowsEnabled: true })
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

  it('does not ask for the list when the schema says there are no flows', async () => {
    useSchemaStore().flowsEnabled = false
    const list = useFlowList()
    await loadFlowList()
    expect(listFlowsMock).not.toHaveBeenCalled()
    expect(list.hasFlows.value).toBe(false)
    expect(list.loaded.value).toBe(true)
  })

  it('waits for the schema before it decides, without loading it', async () => {
    const schema = useSchemaStore()
    schema.loaded = false
    schema.flowsEnabled = false
    const load = vi.spyOn(schema, 'load')
    listFlowsMock.mockResolvedValue([flow])
    const pending = loadFlowList()
    await flushPromises()
    expect(listFlowsMock).not.toHaveBeenCalled()

    schema.flowsEnabled = true
    schema.loaded = true
    await pending
    expect(listFlowsMock).toHaveBeenCalledTimes(1)
    expect(load).not.toHaveBeenCalled()
  })
})
