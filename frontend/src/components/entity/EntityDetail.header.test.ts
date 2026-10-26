import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import EntityActionsMenu from './EntityActionsMenu.vue'
import type { EntityAction } from './entityActions'
import { useSchemaStore } from '@/stores/schema'
import type { Entity } from '@/types'
import type { ViewResponse } from '@/api'

// The desktop header row: conversions share one menu, and a row that does not
// fit beside the title moves its secondary actions into the "⋯" menu.

const fetchViewMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: async () => [],
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/entity/topic/TOPIC-1', name: 'entity' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

let observers: Array<() => void> = []
class FakeResizeObserver {
  constructor(cb: () => void) {
    observers.push(cb)
  }
  observe() {}
  disconnect() {}
}

const conversions = ['initiatief', 'opdracht', 'verantwoordelijkheidsgebied'].map((to) => ({
  name: `topic-naar-${to}`,
  label: `Omzetten naar ${to}`,
  to,
  allowed: true,
}))

function view(): ViewResponse {
  const entry = {
    id: 'TOPIC-1',
    type: 'topic',
    _title: 'Een topic',
    properties: { titel: 'Een topic' },
    _actions: { update: true, delete: true },
    _conversions: conversions,
  } as unknown as Entity
  return { entry, sections: [] }
}

describe('EntityDetail header actions', () => {
  let pinia: ReturnType<typeof createPinia>
  let headerWidth = 2000

  beforeEach(() => {
    observers = []
    headerWidth = 2000
    vi.stubGlobal('ResizeObserver', FakeResizeObserver)
    vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockImplementation(function (
      this: HTMLElement
    ) {
      return this.classList.contains('detail-header') ? headerWidth : 0
    })
    vi.spyOn(HTMLElement.prototype, 'scrollWidth', 'get').mockImplementation(function (
      this: HTMLElement
    ) {
      return this.classList.contains('desktop-actions') ? 900 : 0
    })
    pinia = createPinia()
    setActivePinia(pinia)
    useSchemaStore().entityTypes.set('topic', {
      name: 'topic',
      label: 'Topic',
      properties: { titel: { type: 'string', values: null } },
    } as never)
    fetchViewMock.mockReset().mockResolvedValue(view())
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
  })

  async function mountDetail() {
    const w = mount(EntityDetail, {
      props: { entityType: 'topic', entityId: 'TOPIC-1' },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    return w
  }

  function desktopLabels(w: Awaited<ReturnType<typeof mountDetail>>) {
    return w.findAll('.desktop-actions button, .desktop-actions a').map((b) => b.text().trim())
  }

  it('offers all conversions in one Convert menu', async () => {
    const w = await mountDetail()
    const labels = desktopLabels(w)
    expect(labels.some((l) => l.startsWith('Convert'))).toBe(true)
    expect(labels.some((l) => l.startsWith('Omzetten'))).toBe(false)
  })

  it('lists the conversions in the phone overflow menu', async () => {
    const w = await mountDetail()
    const menu = w.find('.mobile-actions').findComponent(EntityActionsMenu)
    const labels = (menu.props('actions') as EntityAction[]).map((a) => a.label)
    expect(labels).toEqual(expect.arrayContaining(conversions.map((c) => c.label)))
  })

  it('moves secondary actions into the overflow menu when the row does not fit', async () => {
    const w = await mountDetail()
    expect(w.find('.desktop-overflow').exists()).toBe(false)

    headerWidth = 600
    observers.forEach((cb) => cb())
    await flushPromises()

    const labels = desktopLabels(w)
    expect(labels.some((l) => l.startsWith('Delete'))).toBe(false)
    expect(labels.some((l) => l.startsWith('Convert'))).toBe(false)
    const menu = w.find('.desktop-actions').findComponent(EntityActionsMenu)
    const ids = (menu.props('actions') as EntityAction[]).map((a) => a.id)
    expect(ids).toContain('delete')
    expect(ids).toContain('convert:topic-naar-opdracht')
    expect(ids).not.toContain('edit')
  })
})
