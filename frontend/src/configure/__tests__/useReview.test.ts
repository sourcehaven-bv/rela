import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ApiError } from '@/api/errors'
import type { ConfigResult } from '@/api/configure'
import { useConfigDraftStore } from '@/stores/configDraft'
import { isConfigIncomplete } from '../saveGate'
import { useReview } from '../useReview'
import { snapshot } from './fixture'

const api = vi.hoisted(() => ({
  previewConfigure: vi.fn(),
  saveConfigure: vi.fn(),
  finishConfigureMigration: vi.fn(),
}))
vi.mock('@/api/configure', () => api)

const clean: ConfigResult = { version: 'v1', problems: [], saved: false }

function refusal(status: number, kind: string): ApiError {
  return new ApiError(`refused: ${kind}`, {
    kind: 'http',
    status,
    problem: { type: `https://rela.dev/errors/${kind}`, title: `refused: ${kind}`, status },
    original: null,
  })
}

function setup() {
  setActivePinia(createPinia())
  const draft = useConfigDraftStore()
  draft.load(snapshot())
  draft.setAt('schema', ['entities', 'feature'], 'label', 'Epic')
  return { draft, review: useReview() }
}

describe('useReview', () => {
  const reload = vi.fn()
  const realLocation = window.location

  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    vi.resetAllMocks()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...realLocation, reload },
    })
  })
  afterEach(() => {
    Object.defineProperty(window, 'location', { configurable: true, value: realLocation })
  })

  it('clears a conflict once a later check succeeds', async () => {
    const { review } = setup()
    api.previewConfigure.mockRejectedValueOnce(refusal(409, 'conflict'))
    await review.check()
    expect(review.conflict.value).toBe(true)

    api.previewConfigure.mockResolvedValueOnce(clean)
    await review.check()
    expect(review.conflict.value).toBe(false)
    expect(review.canSave.value).toBe(true)
  })

  it.each(['busy', 'migration_not_started'])(
    'keeps Save available after a %s refusal',
    async (kind) => {
      const { review } = setup()
      api.previewConfigure.mockResolvedValueOnce(clean)
      await review.check()

      api.saveConfigure.mockRejectedValueOnce(refusal(409, kind))
      await review.save()
      expect(review.conflict.value).toBe(false)
      expect(review.failure.value).toBe(`refused: ${kind}`)
      expect(review.canSave.value).toBe(true)
    }
  )

  it('treats a version conflict on save as a conflict', async () => {
    const { review } = setup()
    api.previewConfigure.mockResolvedValueOnce(clean)
    await review.check()
    api.saveConfigure.mockRejectedValueOnce(refusal(409, 'conflict'))
    await review.save()
    expect(review.conflict.value).toBe(true)
    expect(review.canSave.value).toBe(false)
  })

  it('drops a check answered after the draft changed', async () => {
    const { draft, review } = setup()
    let answer: (r: ConfigResult) => void = () => {}
    api.previewConfigure.mockReturnValueOnce(new Promise((resolve) => (answer = resolve)))
    const pending = review.check()
    draft.setAt('schema', ['entities', 'feature'], 'label', 'Initiative')
    answer({ ...clean, problems: [{ code: 'x', message: 'old draft' }] })
    await pending
    expect(draft.result).toBeNull()
  })

  it('keeps the finish-migration prompt across a reload until a retry succeeds', async () => {
    const { draft, review } = setup()
    api.saveConfigure.mockResolvedValueOnce({ ...clean, saved: true, incomplete: true })
    await review.save()
    expect(review.incomplete.value).toBe(true)
    expect(draft.reviewOpen).toBe(true)

    // The config-changed event reloads the tab: a new review starts.
    setActivePinia(createPinia())
    const again = useReview()
    expect(again.incomplete.value).toBe(true)

    api.finishConfigureMigration.mockRejectedValueOnce(refusal(409, 'busy'))
    await again.finishMigration()
    expect(isConfigIncomplete()).toBe(true)

    api.finishConfigureMigration.mockResolvedValueOnce(undefined)
    await again.finishMigration()
    expect(isConfigIncomplete()).toBe(false)
    expect(reload).toHaveBeenCalled()
  })
})
