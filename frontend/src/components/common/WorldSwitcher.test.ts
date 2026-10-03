import { describe, expect, it, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import WorldSwitcher from './WorldSwitcher.vue'
import { useSchemaStore } from '@/stores/schema'

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

describe('WorldSwitcher', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('is hidden with fewer than two worlds', async () => {
    const store = useSchemaStore()
    store.worldOrder = ['only']
    store.worlds = new Map([['only', { readable: true }]])
    const { wrapper } = await mountAt('/')
    expect(wrapper.find('[data-testid="world-switcher"]').exists()).toBe(false)
  })

  it('lists worlds in declaration order, disables unreadable ones, and writes ?world=', async () => {
    const store = useSchemaStore()
    store.worldOrder = ['published', 'draft', 'secret']
    store.worlds = new Map([
      ['published', { readable: true, default: true }],
      ['draft', { readable: true }],
      ['secret', { readable: false }],
    ])
    store.defaultWorld = 'published'
    const { wrapper, router } = await mountAt('/list/x')

    const options = wrapper.findAll('option')
    expect(options.map((o) => o.text())).toEqual(['published', 'draft', 'secret'])
    expect(options[2].attributes('disabled')).toBeDefined()

    await wrapper.find('select').setValue('draft')
    await router.isReady()
    await new Promise((r) => setTimeout(r))
    expect(router.currentRoute.value.query.world).toBe('draft')

    await wrapper.find('select').setValue('published')
    await new Promise((r) => setTimeout(r))
    expect(router.currentRoute.value.query.world).toBeUndefined()
  })
})
