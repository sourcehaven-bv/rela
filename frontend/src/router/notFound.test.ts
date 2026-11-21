import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSpaceStore } from '@/stores/space'

vi.mock('@/api/schema', async (orig) => ({
  ...(await orig<typeof import('@/api/schema')>()),
  getSidebar: vi.fn(async () => ({ app: {}, navigation: [] })),
}))

/*
 * BUG-UZGMHIK: an unknown address rendered an empty pane, and `/login` with a
 * session became `/s/<space>/login`, which was empty too.
 */
describe('unknown routes', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useSpaceStore().set({
      spaces: [
        { id: 'portal', label: 'Portal', home: '/dashboard' },
        { id: 'backoffice', label: 'Backoffice', home: '/dashboard' },
      ],
      space: 'portal',
    })
  })

  it('resolves an unknown path, with or without a space, to the not-found page', async () => {
    const { default: router } = await import('./index')
    expect(router.resolve('/s/backoffice/runs').name).toBe('not-found')
    expect(router.resolve('/runs').name).toBe('not-found')
    expect(router.resolve('/s/portal/no/such/page').name).toBe('not-found')
  })

  it('keeps known routes and the bare space ahead of the catch-all', async () => {
    const { default: router } = await import('./index')
    expect(router.resolve('/s/portal/flows/runs').name).toBe('flow-runs')
    expect(router.resolve('/flows/runs').name).toBe('flow-runs')
    expect(router.resolve('/s/portal').name).not.toBe('not-found')
    expect(router.resolve('/').name).not.toBe('not-found')
  })

  it('shows the not-found page in the current space for an unprefixed unknown path', async () => {
    const { default: router } = await import('./index')
    await router.push('/runs')
    expect(router.currentRoute.value.path).toBe('/s/portal/runs')
    expect(router.currentRoute.value.name).toBe('not-found')
  })

  it.each(['/login', '/s/portal/login'])(
    'sends %s with a session to the home page',
    async (path) => {
      const { default: router } = await import('./index')
      await router.push('/s/portal/settings')
      await router.push(path)
      expect(router.currentRoute.value.path).toBe('/s/portal/dashboard')
    }
  )

  it('keeps the space of /s/<space>/login', async () => {
    const { default: router } = await import('./index')
    await router.push('/s/portal/settings')
    await router.push('/s/backoffice/login')
    expect(router.currentRoute.value.path).toBe('/s/backoffice/dashboard')
  })

  it('shows the not-found page under the Configure space too', async () => {
    const { default: router } = await import('./index')
    expect(router.resolve('/configure/no/such/page').name).toBe('not-found')
  })
})
