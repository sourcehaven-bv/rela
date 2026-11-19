import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { SidebarData } from '@/types'

const getSidebar = vi.hoisted(() => vi.fn<(space?: string) => Promise<SidebarData>>())
vi.mock('@/api/schema', async (orig) => ({
  ...(await orig<typeof import('@/api/schema')>()),
  getSidebar,
}))

function sidebar(over: Partial<SidebarData> = {}): SidebarData {
  return { app: { name: 'x', description: '' }, navigation: [], ...over }
}

const twoSpaces = {
  spaces: [
    { id: 'staff', label: 'Staff', home: '/list/accounts' },
    { id: 'portal', label: 'Portal', home: '/p/account' },
  ],
}

/*
 * The app root opens the home of the current space (TKT-JEAR2UA). Each test
 * imports a fresh router, so a guard's redirect in one does not leave the
 * next already at its target. The first navigation to a view loads its
 * component, which can take longer than the default timeout.
 */
describe('root redirect', { timeout: 30_000 }, () => {
  beforeEach(() => {
    vi.resetModules()
    setActivePinia(createPinia())
    localStorage.clear()
    getSidebar.mockReset()
  })

  afterEach(() => localStorage.clear())

  async function openRoot(): Promise<string> {
    const { default: router } = await import('./index')
    await router.push('/')
    return router.currentRoute.value.path
  }

  it('opens the dashboard without spaces', async () => {
    getSidebar.mockResolvedValue(sidebar())
    expect(await openRoot()).toBe('/dashboard')
    expect(getSidebar).toHaveBeenCalledWith(undefined)
  })

  it('opens the home of the space the server resolves', async () => {
    getSidebar.mockResolvedValue(sidebar({ ...twoSpaces, space: 'staff' }))
    expect(await openRoot()).toBe('/s/staff/list/accounts')
  })

  it('asks for the space this browser was last in', async () => {
    const { lastSpaceStorageKey } = await import('@/stores/space')
    localStorage.setItem(lastSpaceStorageKey(), 'portal')
    getSidebar.mockResolvedValue(sidebar({ ...twoSpaces, space: 'portal' }))
    expect(await openRoot()).toBe('/s/portal/p/account')
    expect(getSidebar).toHaveBeenCalledWith('portal')
  })

  it('opens the dashboard when the sidebar request fails', async () => {
    getSidebar.mockRejectedValue(new Error('offline'))
    expect(await openRoot()).toBe('/dashboard')
  })

  it('sends no request once a sidebar without spaces has loaded', async () => {
    const { useSpaceStore } = await import('@/stores/space')
    useSpaceStore().set({})
    expect(await openRoot()).toBe('/dashboard')
    expect(getSidebar).not.toHaveBeenCalled()
  })

  it('opens the current space home once spaces have loaded', async () => {
    const { useSpaceStore } = await import('@/stores/space')
    useSpaceStore().set({ ...twoSpaces, space: 'portal' })
    getSidebar.mockResolvedValue(sidebar({ ...twoSpaces, space: 'portal' }))
    expect(await openRoot()).toBe('/s/portal/p/account')
  })
})
