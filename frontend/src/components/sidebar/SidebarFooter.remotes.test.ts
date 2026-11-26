import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { ref, computed } from 'vue'
import SidebarFooter from './SidebarFooter.vue'
import { useGitStore } from '@/stores'

/**
 * The Remotes link shows only when /_me says the principal may use the page
 * (TKT-0FMYPEZ). The server checks remote:sync on every request either way.
 */

vi.mock('@/composables/useNextAction', () => ({
  useNextAction: () => ({
    suggestion: ref(null),
    expanded: ref(false),
    bandLabel: computed(() => ''),
    isStatusBar: computed(() => false),
    markShown: vi.fn(),
    loadOnce: vi.fn(),
  }),
}))

vi.mock('@/utils/markdown', () => ({ renderMarkdown: (s: string) => s }))

const me = vi.hoisted(() => ({ value: undefined as { can_remote_sync?: boolean } | undefined }))
vi.mock('@/queries/me', async () => {
  const { ref } = await import('vue')
  return { useMe: () => ({ data: ref(me.value) }) }
})

describe('SidebarFooter / remotes link', () => {
  let router: Router
  let wrapper: VueWrapper | null = null

  beforeEach(async () => {
    setActivePinia(createPinia())
    useGitStore().fetchStatus = vi.fn().mockResolvedValue(undefined)
    router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div/>' } }],
    })
    await router.push('/')
    await router.isReady()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
  })

  it.each([
    { name: 'may sync', value: { can_remote_sync: true }, shown: true },
    { name: 'may not sync', value: {}, shown: false },
    { name: 'me not loaded', value: undefined, shown: false },
  ])('$name', async ({ value, shown }) => {
    me.value = value
    wrapper = mount(SidebarFooter, { global: { plugins: [router] } })
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="footer-remotes"]').exists()).toBe(shown)
  })

  it('opens the remotes page', async () => {
    me.value = { can_remote_sync: true }
    wrapper = mount(SidebarFooter, { global: { plugins: [router] } })
    await wrapper.vm.$nextTick()
    await wrapper.find('[data-testid="footer-remotes"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(router.currentRoute.value.path).toBe('/remotes')
  })
})
