import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { ref, computed } from 'vue'
import SidebarFooter from './SidebarFooter.vue'
import { useGitStore, useSchemaStore } from '@/stores'
import type { AppConfig } from '@/types'

/**
 * The Settings and About buttons follow `builtins.settings` and
 * `builtins.about`, and About shows only when it has something besides the
 * app name: a description, a version or a link (`app.about:`).
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

vi.mock('@/queries/me', async () => {
  const { ref } = await import('vue')
  return { useMe: () => ({ data: ref({}) }) }
})

describe('SidebarFooter / Settings and About', () => {
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

  function mountFooter(app: AppConfig, description = '', props: Record<string, boolean> = {}) {
    const schema = useSchemaStore()
    schema.app = app
    schema.aboutDescription = description
    wrapper = mount(SidebarFooter, {
      props,
      global: { plugins: [router], stubs: { teleport: true } },
    })
    return wrapper
  }

  it('hides About when there is only the app name', () => {
    const w = mountFooter({ name: 'Portal' })
    expect(w.find('[data-testid="footer-about"]').exists()).toBe(false)
  })

  it('shows the version and links in About', async () => {
    const about = {
      version: '2026.11',
      links: [
        { label: 'Help', url: 'https://help.example.com/' },
        { label: 'Terms', url: '/terms' },
      ],
    }
    const w = mountFooter({ name: 'Portal', about })
    await w.find('[data-testid="footer-about"]').trigger('click')
    expect(w.find('[data-testid="about-version"]').text()).toBe(`Version ${about.version}`)
    const links = w.findAll('[data-testid="about-links"] a')
    expect(links.map((a) => [a.text(), a.attributes('href')])).toEqual(
      about.links.map((l) => [l.label, l.url])
    )
    expect(links[0].attributes('rel')).toContain('noopener')
  })

  it('shows About for a description alone', () => {
    const w = mountFooter({ name: 'Portal' }, 'Your workspace')
    expect(w.find('[data-testid="footer-about"]').exists()).toBe(true)
  })

  it('hides Settings and About when builtins hide them', () => {
    const w = mountFooter({ name: 'Portal', about: { version: '1' } }, 'Your workspace', {
      showSettings: false,
      showAbout: false,
    })
    expect(w.find('[data-testid="footer-settings"]').exists()).toBe(false)
    expect(w.find('[data-testid="footer-about"]').exists()).toBe(false)
  })

  it('shows Settings by default', () => {
    const w = mountFooter({ name: 'Portal' })
    expect(w.find('[data-testid="footer-settings"]').exists()).toBe(true)
  })
})
