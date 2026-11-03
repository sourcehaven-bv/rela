import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import ConflictsView from './ConflictsView.vue'
import { ApiError } from '@/api/errors'

/**
 * The conflict routes need git:sync (BUG-T6HNKS). A 403 is a missing
 * permission, not a transient failure, so the view says which permission is
 * missing and offers no Retry.
 */

vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn() }),
}))

const getConflictsMock = vi.fn()
vi.mock('@/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api')>()),
  getConflicts: () => getConflictsMock(),
}))

describe('ConflictsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getConflictsMock.mockReset()
  })

  it('names git:sync and offers no retry when the permission is missing', async () => {
    getConflictsMock.mockRejectedValue(new ApiError('Forbidden', {
      kind: 'http',
      status: 403,
      problem: { type: 'https://rela.dev/errors/permission_required', title: 'Forbidden', status: 403 },
      original: null,
    }))
    const w = mount(ConflictsView)
    await flushPromises()

    const region = w.find('[data-testid="conflicts-error"]')
    expect(region.exists()).toBe(true)
    expect(region.text()).toContain('git:sync')
    expect(region.text()).not.toContain('Retry')
  })

  // A 403 that is not the git:sync refusal, such as resolve's write check,
  // must not send the user after a permission they already hold.
  it.each([
    { name: 'a server error', status: 500 },
    { name: 'another 403', status: 403 },
  ])('keeps the server message and the retry for $name', async ({ status }) => {
    getConflictsMock.mockRejectedValue(new ApiError('Boom', { kind: 'http', status, original: null }))
    const w = mount(ConflictsView)
    await flushPromises()

    const region = w.find('[data-testid="conflicts-error"]')
    expect(region.text()).toContain('Boom')
    expect(region.text()).not.toContain('git:sync')
    expect(region.text()).toContain('Retry')
  })
})
