import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  cancelFlowRun,
  completeFlowStep,
  getFlowRun,
  listFlowRuns,
  listFlowTasks,
  listFlows,
  listSubjectRuns,
  retryFlowRun,
  startFlowRun,
} from './flows'
import { api } from './client'

vi.mock('./client', () => ({
  api: {
    get: vi.fn().mockResolvedValue({}),
    post: vi.fn().mockResolvedValue({}),
  },
}))

describe('flow requests', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('reads the flows, and an answer without data as none', async () => {
    vi.mocked(api.get).mockResolvedValueOnce({ data: [{ name: 'onboarding' }] })
    expect(await listFlows()).toEqual([{ name: 'onboarding' }])
    vi.mocked(api.get).mockResolvedValueOnce({})
    expect(await listFlows()).toEqual([])
    expect(api.get).toHaveBeenCalledWith('/_flows', undefined, undefined)
  })

  it('sends only the filters that are set', async () => {
    await listFlowRuns({ flow: 'onboarding', status: '', page: 2, perPage: 25 })
    expect(api.get).toHaveBeenCalledWith(
      '/_flows/runs',
      { flow: 'onboarding', page: 2, per_page: 25 },
      undefined
    )
    await listFlowTasks()
    expect(api.get).toHaveBeenLastCalledWith('/_flows/tasks', {}, undefined)
  })

  it('reads the runs of one subject', async () => {
    vi.mocked(api.get).mockResolvedValueOnce({ runs: [{ id: 'R1' }], total: 1 })
    expect(await listSubjectRuns('PER-1@draft')).toEqual([{ id: 'R1' }])
    expect(api.get).toHaveBeenCalledWith('/_flows/runs', { subject: 'PER-1@draft' }, undefined)
    vi.mocked(api.get).mockResolvedValueOnce({})
    expect(await listSubjectRuns('PER-1')).toEqual([])
  })

  it('escapes the run id in the path', async () => {
    await getFlowRun('a/b')
    expect(api.get).toHaveBeenCalledWith('/_flows/runs/a%2Fb', undefined, undefined)
  })

  it('starts, completes, cancels and retries', async () => {
    vi.mocked(api.post).mockResolvedValueOnce({ id: 'R9' })
    expect(await startFlowRun('onboarding', 'PER-1')).toBe('R9')
    expect(api.post).toHaveBeenCalledWith('/_flows/runs', { flow: 'onboarding', subject: 'PER-1' })

    await completeFlowStep('R9', 3, { documents: true })
    expect(api.post).toHaveBeenCalledWith('/_flows/runs/R9/_complete', {
      generation: 3,
      properties: { documents: true },
    })

    await cancelFlowRun('R9')
    expect(api.post).toHaveBeenCalledWith('/_flows/runs/R9/_cancel', {})
    await retryFlowRun('R9')
    expect(api.post).toHaveBeenCalledWith('/_flows/runs/R9/_retry', {})
  })
})
