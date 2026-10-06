import { describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useConfigDraftStore } from '@/stores/configDraft'
import { useDiscardDraft } from '../useDiscardDraft'
import { snapshot } from './fixture'

const refetch = vi.hoisted(() => vi.fn())
vi.mock('@/queries/configure', () => ({ useConfigureSnapshot: () => ({ refetch }) }))

describe('useDiscardDraft', () => {
  it('drops the draft and fetches the current configuration', async () => {
    setActivePinia(createPinia())
    const draft = useConfigDraftStore()
    draft.load(snapshot())
    draft.setAt('schema', ['entities', 'feature'], 'label', 'Epic')

    await useDiscardDraft()()
    expect(draft.hasDraft).toBe(false)
    expect(refetch).toHaveBeenCalledOnce()
  })
})
