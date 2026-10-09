// Backlinks shown through a view (TKT-HF0D5W): per-type table groups carry
// their own columns, an incomplete collection says so, and the server
// decides whether the built-in "Mentioned by" panel shows.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import { useSchemaStore } from '@/stores/schema'
import type { Entity } from '@/types'
import type { ViewResponse, ViewSection } from '@/api'

const fetchViewMock = vi.fn()

vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: () => Promise.resolve([]),
}))
vi.mock('vue-router', async (orig) => ({
  ...(await orig<typeof import('vue-router')>()),
  useRoute: () => ({
    query: {},
    params: {},
    path: '/entity/feature/F-1',
    fullPath: '/entity/feature/F-1',
  }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}))

const entry: Entity = { id: 'F-1', type: 'feature', properties: { title: 'Feature' } }

const byTypeTable: ViewSection = {
  heading: 'Mentioned by',
  sectionId: 'mentioned-by',
  display: 'table',
  isEmpty: false,
  isGrouped: true,
  hasContent: false,
  columns: [{ property: 'title' }],
  groups: [
    {
      groupName: 'Decisions',
      entityType: 'decision',
      rows: [{ entityId: 'D-1', entityType: 'decision', cells: [{ values: ['Pick a store'] }] }],
    },
    {
      groupName: 'Tickets',
      entityType: 'ticket',
      columns: [{ property: 'title' }, { property: 'status', label: 'State' }],
      rows: [
        {
          entityId: 'T-1',
          entityType: 'ticket',
          cells: [{ values: ['Index it'] }, { values: ['open'] }],
        },
      ],
    },
  ],
}

describe('EntityDetail backlinks through a view', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    fetchViewMock.mockReset()
    const schema = useSchemaStore()
    for (const name of ['feature', 'ticket', 'decision']) {
      schema.entityTypes.set(name, { name, label: name, properties: {} } as never)
    }
    schema.loaded = true
  })

  async function render(resp: Partial<ViewResponse>) {
    fetchViewMock.mockResolvedValue({ entry, sections: [], ...resp })
    const wrapper = mount(EntityDetail, {
      props: { entityType: 'feature', entityId: 'F-1' },
      global: {
        plugins: [pinia, PiniaColada],
        stubs: { DocumentsPanel: true, CommentsPanel: true, MentionedByPanel: true },
      },
    })
    await flushPromises()
    return wrapper
  }

  it('gives each type group its own header', async () => {
    const wrapper = await render({ sections: [byTypeTable] })
    const groups = wrapper.findAll('.table-group')
    expect(groups.map((g) => g.find('h3').text())).toEqual(['Decisions', 'Tickets'])
    expect(groups[0].findAll('th:not(.actions-col)').map((th) => th.text())).toEqual(['title'])
    expect(groups[1].findAll('th:not(.actions-col)').map((th) => th.text())).toEqual([
      'title',
      'State',
    ])
    expect(groups[1].findAll('tbody td')[1].text()).toContain('open')
  })

  it('says when the collection is incomplete', async () => {
    const wrapper = await render({ sections: [{ ...byTypeTable, incomplete: true }] })
    expect(wrapper.find('.section-incomplete').exists()).toBe(true)
  })

  it('shows no note for a complete collection', async () => {
    const wrapper = await render({ sections: [byTypeTable] })
    expect(wrapper.find('.section-incomplete').exists()).toBe(false)
  })

  it('shows the built-in panel unless the server hides it', async () => {
    const shown = await render({})
    expect(shown.find('mentioned-by-panel-stub').exists()).toBe(true)
    const hidden = await render({ hideMentionedByPanel: true })
    expect(hidden.find('mentioned-by-panel-stub').exists()).toBe(false)
  })
})
