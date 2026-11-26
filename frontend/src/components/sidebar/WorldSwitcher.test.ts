import { describe, expect, it, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import WorldSwitcher from './WorldSwitcher.vue'
import { useSchemaStore } from '@/stores/schema'
import type { WorldInfo } from '@/types'

async function mountAt(url: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }],
  })
  await router.push(url)
  await router.isReady()
  const wrapper = mount(WorldSwitcher, { global: { plugins: [router] } })
  return { wrapper, router }
}

function declare(worlds: [string, WorldInfo][], defaultWorld = worlds[0][0]) {
  const store = useSchemaStore()
  store.worldOrder = worlds.map(([name]) => name)
  store.worlds = new Map(worlds)
  store.defaultWorld = defaultWorld
  return store
}

const links = (wrapper: Awaited<ReturnType<typeof mountAt>>['wrapper']) =>
  wrapper.findAll('[data-testid="world-switcher"] .rl-segmented__option')

describe('WorldSwitcher', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('is hidden with fewer than two worlds', async () => {
    declare([['only', { readable: true }]])
    const { wrapper } = await mountAt('/')
    expect(wrapper.find('[data-testid="world-switcher"]').exists()).toBe(false)
  })

  it('renders two or three worlds as links, by label, with the current one marked', async () => {
    declare([
      ['werk', { readable: true, label: 'Concept' }],
      ['vastgesteld', { readable: true }],
    ])
    const { wrapper } = await mountAt('/list/x?world=vastgesteld&page=2')

    const opts = links(wrapper)
    expect(opts.map((o) => o.text())).toEqual(['Concept', 'vastgesteld'])
    expect(opts.map((o) => o.element.tagName)).toEqual(['A', 'A'])
    expect(opts[1].attributes('aria-current')).toBe('true')
    expect(opts[0].attributes('aria-current')).toBeUndefined()
    // The default world drops ?world=; the page resets.
    expect(opts[0].attributes('href')).toBe('/list/x')
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('World')
  })

  it('leaves a hidden world out unless it is the world on screen', async () => {
    declare([
      ['werk', { readable: true }],
      ['vastgesteld', { readable: true }],
      ['publiek', { readable: true, hidden: true }],
    ])
    const first = await mountAt('/list/x')
    expect(links(first.wrapper).map((o) => o.text())).toEqual(['werk', 'vastgesteld'])

    const second = await mountAt('/list/x?world=publiek')
    const opts = links(second.wrapper)
    expect(opts.map((o) => o.text())).toEqual(['werk', 'vastgesteld', 'publiek'])
    expect(opts[2].attributes('aria-current')).toBe('true')
  })

  it('keeps the control type when the reader is on a hidden world', async () => {
    declare([
      ['a', { readable: true }],
      ['b', { readable: true }],
      ['c', { readable: true }],
      ['feed', { readable: true, hidden: true }],
    ])
    const { wrapper } = await mountAt('/?world=feed')
    expect(links(wrapper)).toHaveLength(4)
    expect(wrapper.find('select').exists()).toBe(false)
  })

  it('renders nothing when the operator sets world_switcher: none', async () => {
    const store = declare([
      ['werk', { readable: true }],
      ['vastgesteld', { readable: true }],
    ])
    store.app = { name: 'x', world_switcher: 'none' }
    const { wrapper } = await mountAt('/')
    expect(wrapper.find('[data-testid="world-switcher"]').exists()).toBe(false)
  })

  it('renders nothing when only one world can be selected', async () => {
    declare([
      ['werk', { readable: true }],
      ['geheim', { readable: false }],
    ])
    const { wrapper } = await mountAt('/')
    expect(wrapper.find('[data-testid="world-switcher"]').exists()).toBe(false)
  })

  it('stays as the way back on a world the reader may not read', async () => {
    declare([
      ['werk', { readable: true }],
      ['geheim', { readable: false }],
    ])
    const { wrapper } = await mountAt('/?world=geheim')
    const opts = links(wrapper)
    expect(opts.map((o) => o.text())).toEqual(['werk', 'geheim'])
    expect(opts[1].attributes('aria-disabled')).toBe('true')
    expect(opts[1].attributes('role')).toBe('link')
  })

  it('links the current world to the page as it is, keeping the page number', async () => {
    declare([
      ['werk', { readable: true }],
      ['vastgesteld', { readable: true }],
    ])
    const { wrapper } = await mountAt('/list/x?world=vastgesteld&page=3')
    expect(links(wrapper)[1].attributes('href')).toBe('/list/x?world=vastgesteld&page=3')
  })

  it('uses a select for four or more worlds, disables unreadable ones, and writes ?world=', async () => {
    declare([
      ['published', { readable: true }],
      ['draft', { readable: true, label: 'Draft' }],
      ['review', { readable: true }],
      ['secret', { readable: false }],
    ])
    const { wrapper, router } = await mountAt('/list/x')

    const options = wrapper.findAll('option')
    expect(options.map((o) => o.text())).toEqual(['published', 'Draft', 'review', 'secret'])
    expect(options[3].attributes('disabled')).toBeDefined()
    expect(wrapper.find('select').attributes('aria-label')).toBeTruthy()

    await wrapper.find('select').setValue('draft')
    await new Promise((r) => setTimeout(r))
    expect(router.currentRoute.value.query.world).toBe('draft')

    await wrapper.find('select').setValue('published')
    await new Promise((r) => setTimeout(r))
    expect(router.currentRoute.value.query.world).toBeUndefined()
  })
})
