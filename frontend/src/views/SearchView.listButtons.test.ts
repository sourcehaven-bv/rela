import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import SearchView from './SearchView.vue'
import { useSchemaStore } from '@/stores/schema'
import { resetPilesState } from '@/composables/usePiles'

// "Add results to pile" follows `list_buttons.add_to_pile`, as the list's bulk
// bar does.

const searchEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  searchEntities: (...args: unknown[]) => searchEntitiesMock(...args),
}))

const piles = vi.hoisted(() => ({ listPiles: vi.fn() }))
vi.mock('@/api/piles', async (orig) => ({
  ...(await orig<typeof import('@/api/piles')>()),
  ...piles,
}))

describe('SearchView: Add results to pile', () => {
  let router: Router
  let wrapper: VueWrapper | null = null

  beforeEach(async () => {
    setActivePinia(createPinia())
    resetPilesState()
    piles.listPiles.mockReset().mockResolvedValue({ piles: [], icons: [] })
    searchEntitiesMock.mockReset().mockResolvedValue({
      data: [{ id: 'POL-1', type: 'policy', properties: { title: 'Policy one' } }],
    })
    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/', name: 'home', component: { template: '<div/>' } },
        { path: '/search', name: 'search', component: { template: '<div/>' } },
      ],
    })
    await router.push('/search?q=policy')
    await router.isReady()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    document.body.innerHTML = ''
  })

  async function mountSearch(addToPile: boolean) {
    const schema = useSchemaStore()
    schema.setPiles({ piles_available: true, piles: null })
    schema.setListButtons({ add_to_pile: addToPile })
    wrapper = mount(SearchView, {
      global: { plugins: [router], stubs: { AdHocFilterMenu: true, EntityDetailPanel: true } },
    })
    await flushPromises()
    expect(wrapper.find('.result-item').exists()).toBe(true)
    return wrapper
  }

  it('offers Add results to pile by default', async () => {
    const w = await mountSearch(true)
    expect(w.find('[data-testid="add-to-pile"]').exists()).toBe(true)
  })

  it('leaves it out when list_buttons hides it', async () => {
    const w = await mountSearch(false)
    expect(w.find('[data-testid="add-to-pile"]').exists()).toBe(false)
  })
})
