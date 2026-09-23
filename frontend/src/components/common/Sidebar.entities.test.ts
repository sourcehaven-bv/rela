import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import Sidebar from './Sidebar.vue'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse, SidebarData } from '@/types'

const getSidebarMock = vi.fn()
const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  getSidebar: () => getSidebarMock(),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

// Capture the shared SSE connection so a test can send `refresh`. It is a
// module singleton, so the first instance serves every test in this file.
let source: { _emit: (type: string, data?: string) => void } | null = null
const BaseEventSource = globalThis.EventSource as unknown as new (url: string) => EventSource
vi.stubGlobal(
  'EventSource',
  class extends BaseEventSource {
    constructor(url: string) {
      super(url)
      source = this as unknown as typeof source
    }
  }
)

function sidebar(navigation: SidebarData['navigation']): SidebarData {
  return { app: { name: 'Test' }, navigation } as SidebarData
}

function rows(ids: string[], total = ids.length): ListResponse<Entity> {
  const data: Entity[] = ids.map((id) => ({
    id,
    type: 'project',
    properties: {},
    _title: `Title ${id}`,
  }))
  return { data, meta: { total, page: 1, per_page: 100, has_more: false }, included: {} }
}

/**
 * Sidebar rendering of an `entities:` entry (TKT-PEKL8L): the rows become
 * links under the group heading, and a group whose only entries matched
 * nothing hides its heading.
 */
describe('Sidebar / entities entries', () => {
  let router: Router
  let wrapper: VueWrapper | null = null

  beforeEach(async () => {
    getSidebarMock.mockReset()
    listEntitiesMock.mockReset()
    _setEntityPluralForTest('project', 'projects')
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
  })

  async function mountSidebar() {
    const pinia = createPinia()
    setActivePinia(pinia)
    wrapper = mount(Sidebar, { global: { plugins: [pinia, PiniaColada, router] } })
    await flushPromises()
    await flushPromises()
    return wrapper
  }

  // v-show toggles inline display; isVisible() would need an attached document.
  const hidden = (el: { attributes: (n: string) => string | undefined }) =>
    (el.attributes('style') ?? '').includes('display: none')

  it('renders one link per row under the group heading', async () => {
    getSidebarMock.mockResolvedValue(
      sidebar([
        { group: 'Projects', items: [{ label: '', icon: 'file', entities: { type: 'project' } }] },
      ])
    )
    listEntitiesMock.mockResolvedValue(rows(['PRJ-1', 'PRJ-2'], 105))
    const w = await mountSidebar()

    const section = w.findAll('.nav-section').find((s) => s.text().includes('Projects'))
    expect(section).toBeDefined()
    const links = section!.findAll('a.nav-item')
    expect(links.map((l) => l.text())).toEqual(['Title PRJ-1', 'Title PRJ-2'])
    expect(links[0].attributes('href')).toContain('PRJ-1')
    expect(section!.text()).toContain('and 103 more')
  })

  it('hides a group whose only entries matched nothing', async () => {
    getSidebarMock.mockResolvedValue(
      sidebar([{ group: 'Projects', items: [{ label: '', entities: { type: 'project' } }] }])
    )
    listEntitiesMock.mockResolvedValue(rows([]))
    const w = await mountSidebar()

    const section = w.findAll('.nav-section').find((s) => s.text().includes('Projects'))
    expect(section).toBeDefined()
    expect(hidden(section!)).toBe(true)
  })

  it('keeps the heading hidden while the first fetch is pending', async () => {
    getSidebarMock.mockResolvedValue(
      sidebar([{ group: 'Projects', items: [{ label: '', entities: { type: 'project' } }] }])
    )
    listEntitiesMock.mockReturnValue(new Promise(() => {}))
    const w = await mountSidebar()

    const section = w.findAll('.nav-section').find((s) => s.text().includes('Projects'))
    expect(hidden(section!)).toBe(true)
  })

  it('keeps a group visible when it also holds an ordinary link', async () => {
    getSidebarMock.mockResolvedValue(
      sidebar([
        {
          group: 'Projects',
          items: [
            { label: 'All projects', href: '/list/projects' },
            { label: '', entities: { type: 'project' } },
          ],
        },
      ])
    )
    listEntitiesMock.mockResolvedValue(rows([]))
    const w = await mountSidebar()

    const section = w.findAll('.nav-section').find((s) => s.text().includes('Projects'))
    expect(hidden(section!)).toBe(false)
  })

  it('shows a load failure rather than hiding the group', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    getSidebarMock.mockResolvedValue(
      sidebar([{ group: 'Projects', items: [{ label: '', entities: { type: 'project' } }] }])
    )
    listEntitiesMock.mockRejectedValue(new Error('boom'))
    const w = await mountSidebar()

    const section = w.findAll('.nav-section').find((s) => s.text().includes('Projects'))
    expect(hidden(section!)).toBe(false)
    expect(section!.find('[role="status"]').text()).toBe('Could not load')
  })

  it('re-hides an empty group after the sidebar reloads', async () => {
    const payload = sidebar([
      { group: 'Projects', items: [{ label: '', entities: { type: 'project' } }] },
    ])
    getSidebarMock.mockResolvedValue(payload)
    listEntitiesMock.mockResolvedValue(rows([]))
    const w = await mountSidebar()

    // A config reload replaces the payload; the cached query result does not
    // change, so the entry must still report itself empty.
    getSidebarMock.mockResolvedValue(structuredClone(payload))
    source!._emit('refresh')
    await flushPromises()
    expect(getSidebarMock).toHaveBeenCalledTimes(2)

    const section = w.findAll('.nav-section').find((s) => s.text().includes('Projects'))
    expect(hidden(section!)).toBe(true)
  })

  it('ignores a sidebar response overtaken by a later load', async () => {
    let resolveFirst: (d: SidebarData) => void = () => {}
    getSidebarMock.mockReturnValueOnce(new Promise((r) => (resolveFirst = r)))
    listEntitiesMock.mockResolvedValue(rows(['PRJ-1']))
    const w = await mountSidebar()

    getSidebarMock.mockResolvedValueOnce(
      sidebar([{ group: 'Newer', items: [{ label: '', entities: { type: 'project' } }] }])
    )
    source!._emit('refresh')
    await flushPromises()
    resolveFirst(
      sidebar([{ group: 'Older', items: [{ label: '', entities: { type: 'project' } }] }])
    )
    await flushPromises()

    const titles = w.findAll('.nav-section-title').map((t) => t.text())
    expect(titles).toContain('Newer')
    expect(titles).not.toContain('Older')
  })
})
