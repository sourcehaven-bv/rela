import { describe, it, expect, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSpaceStore } from '@/stores/space'

vi.mock('@/api/schema', async (orig) => ({
  ...(await orig<typeof import('@/api/schema')>()),
  getSidebar: vi.fn(async () => ({ app: {}, navigation: [] })),
}))

/*
 * An unprefixed path is sent into the current space. A push stays a push, so
 * the page it left is still one Back away: a form opened with `e` from a list
 * returns to that list on Cancel.
 */
describe('space redirect', () => {
  it('keeps the page a redirected push came from in history', async () => {
    setActivePinia(createPinia())
    useSpaceStore().set({
      spaces: [{ id: 'iso', label: 'ISMS', home: '/dashboard' }],
      space: 'iso',
    })
    const { default: router } = await import('./index')

    await router.push('/s/iso/about')
    await router.push('/settings')
    expect(router.currentRoute.value.path).toBe('/s/iso/settings')
    expect((router.options.history.state as { back?: string }).back).toBe('/s/iso/about')
  })
})
