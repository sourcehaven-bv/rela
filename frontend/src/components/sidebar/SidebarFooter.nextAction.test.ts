import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { ref, computed } from 'vue'
import SidebarFooter from './SidebarFooter.vue'

/**
 * The next-action chip opens `RlPopover`, which owns its own open state while
 * `useNextAction` owns the authoritative flag. These pin the two directions of
 * that bridge, because nothing else can see it: the composable clears its flag
 * on a route change and after an offer is acted on, and the popover closes
 * itself on Escape and on an outside click. Either side going quiet leaves a
 * chip that claims to be open over no panel, or a panel that will not reopen.
 */

const expanded = ref(false)
const suggestion = ref<Record<string, unknown> | null>(null)
const markShown = vi.fn()
const loadOnce = vi.fn()

vi.mock('@/composables/useNextAction', () => ({
  useNextAction: () => ({
    suggestion,
    expanded,
    bandLabel: computed(() => 'Blocking'),
    isStatusBar: computed(() => !!suggestion.value),
    markShown,
    loadOnce,
  }),
}))

vi.mock('@/utils/markdown', () => ({ renderMarkdown: (s: string) => s }))

describe('SidebarFooter / next-action popover', () => {
  let router: Router
  let wrapper: VueWrapper | null = null

  beforeEach(async () => {
    setActivePinia(createPinia())
    expanded.value = false
    suggestion.value = {
      band: 'blocking',
      source: 'test',
      entity_id: 'TKT-1',
      message: 'Unblock TKT-1',
      actions: [],
    }
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
    vi.clearAllMocks()
  })

  function mountFooter() {
    wrapper = mount(SidebarFooter, { global: { plugins: [router] } })
    return wrapper
  }

  const panel = () => document.body.querySelector('[role="dialog"]')

  it('opens the panel when the composable raises its flag', async () => {
    const w = mountFooter()
    expect(panel()).toBeNull()

    expanded.value = true
    await w.vm.$nextTick()
    await w.vm.$nextTick()

    expect(panel()).not.toBeNull()
    expect(panel()?.textContent).toContain('Unblock TKT-1')
  })

  // The composable clears the flag on a route change and after an offer runs.
  // Neither is something the popover can observe, so it has to be pushed.
  it('closes the panel when the composable lowers its flag', async () => {
    const w = mountFooter()
    expanded.value = true
    await w.vm.$nextTick()
    await w.vm.$nextTick()
    expect(panel()).not.toBeNull()

    expanded.value = false
    await w.vm.$nextTick()
    await w.vm.$nextTick()

    expect(panel()).toBeNull()
  })

  // The other direction: Escape and an outside click are the popover's own, so
  // the flag has to follow them or the chip would refuse to reopen.
  it('lowers the composable flag when the popover closes itself', async () => {
    const w = mountFooter()
    expanded.value = true
    await w.vm.$nextTick()
    await w.vm.$nextTick()
    expect(panel()).not.toBeNull()

    // Escape is bound to the panel, not to the document: the overlay stack is
    // what decides which open overlay a press reaches.
    panel()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await w.vm.$nextTick()

    expect(expanded.value).toBe(false)
  })

  // The operator hooks documented in docs/data-entry.md. One custom.js and one
  // custom.css must serve this surface and the page-level card both, so the
  // classes and the three data attributes are a contract, not styling.
  it('emits the operator hooks on the panel', async () => {
    const w = mountFooter()
    expanded.value = true
    await w.vm.$nextTick()
    await w.vm.$nextTick()

    const hook = document.body.querySelector('.rela-na')
    expect(hook).not.toBeNull()
    expect(hook?.getAttribute('data-band')).toBe('blocking')
    expect(hook?.getAttribute('data-prominence')).toBe('statusbar')
    expect(hook?.getAttribute('data-source')).toBe('test')
    expect(hook?.querySelector('rela-slot[name="companion"]')).not.toBeNull()
    expect(hook?.querySelector('.rela-na-band')).not.toBeNull()
    expect(hook?.querySelector('.rela-na-message')).not.toBeNull()
  })
})
