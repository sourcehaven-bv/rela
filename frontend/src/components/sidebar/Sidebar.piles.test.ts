import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import { defineComponent } from 'vue'
import type { SidebarData } from '@/types'
import { resetFlyout, useFlyout } from '@/composables/useFlyout'
import { resetPilesState, usePiles } from '@/composables/usePiles'

let sidebar: SidebarData

vi.mock('@/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api')>()),
  getSidebar: vi.fn(async () => sidebar),
}))
vi.mock('@/composables/useNavItems', () => ({
  useNavItems: () => ({ itemsFor: () => ({ entries: [] }) }),
}))
const listPiles = vi.hoisted(() => vi.fn())
vi.mock('@/api/piles', async (orig) => ({
  ...(await orig<typeof import('@/api/piles')>()),
  listPiles,
}))

const DialogStub = defineComponent({
  name: 'NewPileDialog',
  emits: ['close'],
  template: '<div data-testid="new-pile-stub" />',
})

import Sidebar from './Sidebar.vue'

const friday = {
  id: 'PIL-AAAA1111',
  name: 'Friday review',
  icon: 'star',
  count: 3,
  created: '',
  updated: '',
}

/**
 * The Piles group (TKT-K3RJLH): present only when the boot payload says piles
 * are available, one row per pile that opens the pile in the flyout, and a
 * "+" that starts a new pile.
 */
describe('Sidebar piles', () => {
  let router: Router
  let wrapper: VueWrapper | null = null

  beforeEach(async () => {
    sidebar = {
      app: { name: 'Atlas' },
      navigation: [],
      piles_available: true,
      piles: null,
    } as SidebarData
    listPiles.mockReset().mockResolvedValue({ piles: [friday], icons: ['star'] })
    resetPilesState()
    resetFlyout()
    router = createRouter({
      history: createWebHistory(),
      routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }],
    })
    await router.push('/')
    await router.isReady()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    document.body.innerHTML = ''
  })

  async function mountSidebar() {
    wrapper = mount(Sidebar, {
      global: { plugins: [router], stubs: { NewPileDialog: DialogStub } },
    })
    await flushPromises()
    await flushPromises()
    return wrapper
  }

  it('has no Piles group when piles are not available', async () => {
    sidebar = { ...sidebar, piles_available: false }
    const w = await mountSidebar()
    expect(w.text()).not.toContain('Piles')
    expect(listPiles).not.toHaveBeenCalled()
  })

  it('has no empty Piles heading until the user has a pile', async () => {
    listPiles.mockResolvedValue({ piles: [], icons: ['star'] })
    const w = await mountSidebar()
    expect(listPiles).toHaveBeenCalled()
    expect(w.text()).not.toContain('Piles')
    expect(w.find('button[aria-label="New pile"]').exists()).toBe(false)

    // The first pile, made from Add to pile, brings the group in.
    listPiles.mockResolvedValue({ piles: [friday], icons: ['star'] })
    await usePiles().refresh()
    await flushPromises()
    expect(w.text()).toContain('Piles')
    expect(w.find('button[aria-label="New pile"]').exists()).toBe(true)
  })

  it('keeps the Piles group when every pile is placed in the navigation', async () => {
    sidebar = {
      ...sidebar,
      navigation: [{ group: 'Mine', items: [{ label: friday.name, pile: friday.name }] }],
    }
    const w = await mountSidebar()
    // The placed pile shows once, in its own group, yet Piles and its "+" stay.
    expect(w.text().split(friday.name)).toHaveLength(2)
    expect(w.text()).toContain('Piles')
    expect(w.find('button[aria-label="New pile"]').exists()).toBe(true)
  })

  it('opens a pile in the flyout, and closes it on a second click', async () => {
    const w = await mountSidebar()
    expect(w.text()).toContain('Piles')
    const row = w.findAll('button').find((b) => b.text().includes('Friday review'))
    expect(row).toBeDefined()

    await row!.trigger('click')
    expect(useFlyout().pile.value?.pileId).toBe(friday.id)
    await row!.trigger('click')
    expect(useFlyout().pile.value).toBeNull()
  })

  it('starts a new pile from the group heading', async () => {
    const w = await mountSidebar()
    expect(w.findComponent(DialogStub).exists()).toBe(false)
    await w.get('button[aria-label="New pile"]').trigger('click')
    const dialog = w.getComponent(DialogStub)
    dialog.vm.$emit('close')
    await flushPromises()
    expect(w.findComponent(DialogStub).exists()).toBe(false)
  })
})
