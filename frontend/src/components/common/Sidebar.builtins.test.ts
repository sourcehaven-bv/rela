import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import { createPinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import type { SidebarData } from '@/types'
import type { Flow } from '@/api/flows'
import { resetFlowList } from '@/composables/useFlowList'

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
  diagram: '',
}
vi.mock('@/api/flows', async (orig) => ({
  ...(await orig<typeof import('@/api/flows')>()),
  listFlows: vi.fn(async () => [signup]),
}))

import Sidebar from './Sidebar.vue'

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

  async function mountSidebar() {
    wrapper = mount(Sidebar, {
      global: { plugins: [createPinia(), PiniaColada, router] },
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
})
