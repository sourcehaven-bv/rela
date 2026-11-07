import { describe, expect, it, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import FaceToggle from './FaceToggle.vue'
import { useSchemaStore } from '@/stores/schema'

async function mountAt(url: string, current = 'concept') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }],
  })
  await router.push(url)
  await router.isReady()
  const wrapper = mount(FaceToggle, {
    props: {
      entityType: 'verwerking',
      current,
      options: [
        { face: 'concept', label: 'Concept', ref: 'VW-1@concept' },
        { face: 'vastgesteld', label: 'Vastgesteld', ref: 'VW-1@vastgesteld' },
        { face: 'en', label: 'English', ref: 'VW-1@en' },
      ],
    },
    global: { plugins: [router] },
  })
  return wrapper
}

describe('FaceToggle', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const store = useSchemaStore()
    store.worldOrder = ['werk', 'vastgesteld']
    store.worlds = new Map([
      ['werk', { readable: true, select: ['concept'] }],
      ['vastgesteld', { readable: true, select: ['vastgesteld'] }],
    ])
    store.defaultWorld = 'werk'
  })

  it('marks the face on screen and links each face to its address', async () => {
    const wrapper = await mountAt('/entity/verwerking/VW-1@concept?from=list')
    const links = wrapper.findAll('a')
    expect(links.map((l) => l.text())).toEqual(['Concept', 'Vastgesteld', 'English'])
    expect(links[0].attributes('aria-current')).toBe('true')
    expect(wrapper.attributes('role')).toBe('group')
    expect(wrapper.attributes('aria-label')).toBeTruthy()
  })

  it('switches to the world that heads the face, and keeps the world when none does', async () => {
    const wrapper = await mountAt('/entity/verwerking/VW-1@concept?from=list')
    const hrefs = wrapper.findAll('a').map((l) => l.attributes('href'))
    // werk is the default world, so it needs no parameter.
    expect(hrefs[0]).toBe('/entity/verwerking/VW-1@concept?from=list')
    expect(hrefs[1]).toBe('/entity/verwerking/VW-1@vastgesteld?from=list&world=vastgesteld')
    // No world heads `en`: the query is kept as it is.
    expect(hrefs[2]).toBe('/entity/verwerking/VW-1@en?from=list')
  })

  it('keeps the current world when the reader may not read the target world', async () => {
    useSchemaStore().worlds.set('vastgesteld', { readable: false, select: ['vastgesteld'] })
    const wrapper = await mountAt('/entity/verwerking/VW-1@concept?from=list')
    expect(wrapper.findAll('a')[1].attributes('href')).toBe('/entity/verwerking/VW-1@vastgesteld?from=list')
  })

  it('links the face on screen to the page as it is', async () => {
    const wrapper = await mountAt('/entity/verwerking/VW-1@vastgesteld?world=werk&from=list', 'vastgesteld')
    expect(wrapper.findAll('a')[1].attributes('href')).toBe('/entity/verwerking/VW-1@vastgesteld?world=werk&from=list')
  })
})
