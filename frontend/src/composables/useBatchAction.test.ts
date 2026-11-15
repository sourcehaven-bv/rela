import { setActivePinia, createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ActionConfig } from '@/types'
import { runBatchAction } from '@/api/actions'
import { useUIStore } from '@/stores'

const confirm = vi.fn()
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ confirm }) }))
vi.mock('@/api/actions', () => ({ runBatchAction: vi.fn() }))

import { useBatchAction } from './useBatchAction'

const action: ActionConfig = { label: 'Triage', llm: { batch: { max: 2 } } }
const batch = { proposals: [], failed: [], skipped: 0 }
const tooMany = (n: number, max: number) => `${n} > ${max}`

describe('useBatchAction', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('refuses more entities than batch.max without asking or sending', async () => {
    const errorSpy = vi.spyOn(useUIStore(), 'error')
    const run = useBatchAction()
    await run.start('triage', action, 'ticket', ['a', 'b', 'c'], tooMany)
    expect(errorSpy).toHaveBeenCalledWith('3 > 2')
    expect(confirm).not.toHaveBeenCalled()
    expect(runBatchAction).not.toHaveBeenCalled()
  })

  it('sends nothing when the user declines', async () => {
    confirm.mockResolvedValue(false)
    const run = useBatchAction()
    await run.start('triage', action, 'ticket', ['a', 'b'], tooMany)
    expect(runBatchAction).not.toHaveBeenCalled()
    expect(run.result.value).toBeNull()
  })

  it('asks with the operator confirm text and starts one run per click', async () => {
    let answer: (ok: boolean) => void = () => {}
    confirm.mockReturnValue(new Promise<boolean>((r) => (answer = r)))
    vi.mocked(runBatchAction).mockResolvedValue(batch)
    const run = useBatchAction()
    const withConfirm = { ...action, confirm: 'This sends ticket bodies to the provider.' }
    const first = run.start('triage', withConfirm, 'ticket', ['a'], tooMany)
    await run.start('triage', withConfirm, 'ticket', ['a'], tooMany)
    answer(true)
    await first
    expect(confirm).toHaveBeenCalledTimes(1)
    expect(confirm.mock.calls[0]?.[0].message).toBe('This sends ticket bodies to the provider.')
    expect(runBatchAction).toHaveBeenCalledTimes(1)
  })

  it('keeps the reply for the dialog once confirmed', async () => {
    confirm.mockResolvedValue(true)
    vi.mocked(runBatchAction).mockResolvedValue(batch)
    const run = useBatchAction()
    await run.start('triage', action, 'ticket', ['a', 'b'], tooMany)
    expect(confirm.mock.calls[0]?.[0].message).toBe('Send 2 entities to the AI provider?')
    expect(runBatchAction).toHaveBeenCalledWith('triage', ['a', 'b'])
    expect(run.result.value).toEqual({ batch, label: 'Triage', entityType: 'ticket' })
    run.close()
    expect(run.result.value).toBeNull()
  })
})
