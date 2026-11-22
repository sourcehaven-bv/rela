import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import { createPinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import type { SidebarData } from '@/types'
import type { Flow } from '@/api/flows'
import { resetFlowList } from '@/composables/useFlowList'
import { useSchemaStore } from '@/stores/schema'

let sidebar: SidebarData

vi.mock('@/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api')>()),
  getSidebar: vi.fn(async () => sidebar),
}))
vi.mock('@/composables/useNavItems', () => ({
  useNavItems: () => ({ itemsFor: () => ({ entries: [] }) }),
}))
const signup: Flow = {
  name: 'signup',
  label: 'Signup',
  subject: 'account',
  version: '1',
  manual: true,
  per_subject: 'many',
  diagram: '',
}
vi.mock('@/api/flows', async (orig) => ({
  ...(await orig<typeof import('@/api/flows')>()),
  listFlows: vi.fn(async () => [signup]),
}))

import Sidebar from './Sidebar.vue'
import { getSidebar } from '@/api'

/**
 * The pinned rows follow the sidebar's `hidden_builtins`, which the server
 * derives from `builtins:` in data-entry.yaml.
 */
describe('Sidebar built-in rows', () => {
  let router: Router
  let wrapper: VueWrapper | null = null

  beforeEach(async () => {
    sidebar = { app: { name: 'Atlas' }, navigation: [] } as unknown as SidebarData
    resetFlowList()
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
    vi.restoreAllMocks()
  })

  async function mountSidebar(asOf = false) {
    const pinia = createPinia()
    // A loaded schema of a project with flows (capabilities.flows).
    Object.assign(useSchemaStore(pinia), { loaded: true, flowsEnabled: true })
    if (asOf) {
      useSchemaStore(pinia).asOfEnabled = true
    }
    wrapper = mount(Sidebar, {
      global: { plugins: [pinia, PiniaColada, router] },
    })
    await flushPromises()
    await flushPromises()
    return wrapper
  }

  function pinnedHrefs(w: VueWrapper): string[] {
    return w.findAll('.rl-sidebar__pinned a').map((a) => a.attributes('href') ?? '')
  }

  it('shows every row without hidden_builtins', async () => {
    const w = await mountSidebar()
    expect(pinnedHrefs(w)).toEqual(['/search', '/flows/tasks', '/flows/runs', '/analyze'])
  })

  it('leaves out the rows the server hides', async () => {
    sidebar.hidden_builtins = ['search', 'flow_runs']
    const w = await mountSidebar()
    expect(pinnedHrefs(w)).toEqual(['/flows/tasks', '/analyze'])
  })

  it('drops the pinned band when every row is hidden', async () => {
    sidebar.hidden_builtins = ['search', 'flow_tasks', 'flow_runs', 'analysis']
    const w = await mountSidebar()
    expect(w.find('.rl-sidebar__pinned').exists()).toBe(false)
  })

  it('keeps the / shortcut off the hidden Search page', async () => {
    sidebar.hidden_builtins = ['search']
    await mountSidebar()
    const push = vi.spyOn(router, 'push')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true }))
    expect(push).not.toHaveBeenCalled()
  })

  function hasShortcutsButton(w: VueWrapper): boolean {
    return w.find('.shortcuts-btn').exists()
  }

  // A fake matchMedia that answers the phone query with `phone`.
  function screen(phone: boolean) {
    vi.spyOn(window, 'matchMedia').mockImplementation(
      (query: string) =>
        ({
          matches: phone && query.includes('max-width'),
          media: query,
          addEventListener: () => {},
          removeEventListener: () => {},
        }) as unknown as MediaQueryList
    )
  }

  it('shows the Shortcuts button by default, also on a phone', async () => {
    screen(true)
    const w = await mountSidebar()
    expect(hasShortcutsButton(w)).toBe(true)
  })

  it('hides the Shortcuts button when shortcuts is hidden', async () => {
    sidebar.hidden_builtins = ['shortcuts']
    const w = await mountSidebar()
    expect(hasShortcutsButton(w)).toBe(false)
    expect(pinnedHrefs(w)).toEqual(['/search', '/flows/tasks', '/flows/runs', '/analyze'])
  })

  it('hides a hidden_on_phone row only on a phone-sized screen', async () => {
    sidebar.phone_hidden_builtins = ['shortcuts', 'analysis']

    screen(false)
    const wide = await mountSidebar()
    expect(hasShortcutsButton(wide)).toBe(true)
    expect(pinnedHrefs(wide)).toEqual(['/search', '/flows/tasks', '/flows/runs', '/analyze'])
    wide.unmount()
    vi.restoreAllMocks()

    screen(true)
    const phone = await mountSidebar()
    expect(hasShortcutsButton(phone)).toBe(false)
    expect(pinnedHrefs(phone)).toEqual(['/search', '/flows/tasks', '/flows/runs'])
  })

  it('shows the as-of picker unless as_of is hidden', async () => {
    const shown = await mountSidebar(true)
    expect(shown.find('[data-testid="as-of-picker"]').exists()).toBe(true)
    shown.unmount()

    sidebar.hidden_builtins = ['as_of']
    const hidden = await mountSidebar(true)
    expect(hidden.find('[data-testid="as-of-picker"]').exists()).toBe(false)
    // The pinned rows are not affected.
    expect(pinnedHrefs(hidden)).toEqual(['/search', '/flows/tasks', '/flows/runs', '/analyze'])
  })
  // BUG-UZGMHIK: on a full load the generic rows showed for a moment before
  // the space's own navigation arrived.
  it('draws no nav rows until the sidebar has loaded', async () => {
    let answer: (d: SidebarData) => void = () => {}
    vi.mocked(getSidebar).mockImplementationOnce(() => new Promise((r) => (answer = r)))
    sidebar.navigation = [{ group: 'Work', items: [{ label: 'Orders', href: '/list/orders' }] }]
    const w = await mountSidebar()
    expect(w.find('#main-sidebar').exists()).toBe(true)
    expect(pinnedHrefs(w)).toEqual([])
    expect(w.text()).not.toContain('Orders')

    answer(sidebar)
    await flushPromises()
    expect(pinnedHrefs(w)).toEqual(['/search', '/flows/tasks', '/flows/runs', '/analyze'])
    expect(w.text()).toContain('Orders')
  })

  it('shows the built-in rows when the sidebar fails to load', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.mocked(getSidebar).mockRejectedValueOnce(new Error('down'))
    const w = await mountSidebar()
    expect(pinnedHrefs(w)).toEqual(['/search', '/flows/tasks', '/flows/runs', '/analyze'])
  })
})
