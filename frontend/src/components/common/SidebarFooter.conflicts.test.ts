import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { ref, computed } from 'vue'
import SidebarFooter from './SidebarFooter.vue'
import { useGitStore } from '@/stores'

/**
 * The conflict routes need git:sync (BUG-T6HNKS), so the "Resolve conflicts"
 * link is offered only when the git status says a sync would be accepted.
 * Otherwise the link would lead to a page that can only answer 403.
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

describe('SidebarFooter / resolve-conflicts link', () => {
  let router: Router
  let wrapper: VueWrapper | null = null

  beforeEach(async () => {
    setActivePinia(createPinia())
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

  async function mountWithStatus(conflict: boolean, canSync: boolean) {
    const git = useGitStore()
    // Keep onMounted from replacing the seeded status with a real request.
    git.fetchStatus = vi.fn().mockResolvedValue(undefined)
    git.status = {
      available: true,
      branch: 'main',
      local_changes: 0,
      remote_ahead: 0,
      syncing: false,
      conflict,
      can_sync: canSync,
    }
    wrapper = mount(SidebarFooter, { global: { plugins: [router] } })
    await wrapper.vm.$nextTick()
    return wrapper
  }

  const link = (w: VueWrapper) => w.find('a[href="/conflicts"]')

  it.each([
    { name: 'conflict and git:sync', conflict: true, canSync: true, shown: true },
    { name: 'conflict without git:sync', conflict: true, canSync: false, shown: false },
    { name: 'no conflict', conflict: false, canSync: true, shown: false },
  ])('$name', async ({ conflict, canSync, shown }) => {
    const w = await mountWithStatus(conflict, canSync)
    expect(link(w).exists()).toBe(shown)
  })
})
