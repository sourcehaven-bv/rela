import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import { useSchemaStore } from '@/stores/schema'
import type { Entity } from '@/types'
import type { ViewResponse } from '@/api'

// An owned entity is shown as part of its owner (TKT-QO14GB). The full page
// replaces the route with the owner's page, anchored at the entity; an
// embedded preview stays put and links to the owner instead.

const fetchViewMock = vi.fn()
const getCommandsMock = vi.fn()

vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: (...a: unknown[]) => getCommandsMock(...a),
}))

// Captures the delete confirm's options without rendering a modal.
const confirmMock = vi.fn()
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: confirmMock }),
  withConfirmError: (fn: unknown) => fn,
}))

const routerReplace = vi.fn()
const mockRoute = {
  query: { from: 'list', list: 'steps', world: 'nl' } as Record<string, unknown>,
  path: '/entity/task/TASK-2',
  name: 'entity',
}
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: routerReplace }),
  useRoute: () => mockRoute,
  RouterLink: {
    props: ['to'],
    template: '<a :href="to"><slot /></a>',
  },
}))

function view(over: Partial<Entity> = {}): ViewResponse {
  return {
    entry: {
      id: 'TASK-2',
      type: 'task',
      _title: 'Book venue',
      properties: { title: 'Book venue' },
      content: '',
      _actions: {},
      ...over,
    },
    sections: [],
  }
}

const owner = { id: 'TASK-1', type: 'task', title: 'Plan launch', relation: 'subtask' }

describe('EntityDetail owned entity', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    useSchemaStore().entityTypes.set('task', {
      name: 'task',
      label: 'Task',
      properties: { title: { type: 'string', values: null } },
    } as never)
    fetchViewMock.mockReset()
    getCommandsMock.mockReset().mockResolvedValue([])
    routerReplace.mockReset().mockResolvedValue(undefined)
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  async function mountDetail(resp: ViewResponse, props: Record<string, unknown> = {}) {
    fetchViewMock.mockResolvedValue(resp)
    const wrapper = mount(EntityDetail, {
      props: { entityType: 'task', entityId: 'TASK-2', ...props },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    return wrapper
  }

  it("replaces the route with the owner's page, anchored at the entity, keeping only the world", async () => {
    const w = await mountDetail(view({ _owner: owner }))
    expect(routerReplace).toHaveBeenCalledWith({
      path: '/entity/task/TASK-1',
      query: { world: 'nl' },
      hash: '#TASK-2',
    })
    // Nothing of the owned entity's own page renders before the redirect.
    expect(w.text()).not.toContain('Book venue')
  })

  it('stays put in a preview and links to the owner', async () => {
    const w = await mountDetail(view({ _owner: owner }), { followOwner: false })
    expect(routerReplace).not.toHaveBeenCalled()
    expect(w.text()).toContain('Book venue')
    const link = w.find('a.owner-link')
    expect(link.exists()).toBe(true)
    expect(link.attributes('href')).toBe('/entity/task/TASK-1#TASK-2')
    expect(link.text()).toBe('Part of Plan launch')
  })

  it('renders an entity without an owner as itself', async () => {
    const w = await mountDetail(view())
    expect(routerReplace).not.toHaveBeenCalled()
    expect(w.text()).toContain('Book venue')
    expect(w.find('a.owner-link').exists()).toBe(false)
  })

  it('gives each section row an anchor at its entity id', async () => {
    const w = await mountDetail({
      ...view(),
      sections: [
        {
          heading: 'Subtasks',
          sectionId: 'subtasks',
          display: 'list',
          isEmpty: false,
          isGrouped: false,
          hasContent: false,
          entities: [{ id: 'TASK-9', type: 'task', title: 'Order catering', hasContent: false }],
        },
      ],
    })
    expect(w.find('#TASK-9').exists()).toBe(true)
  })

  it('renders a related section as rows', async () => {
    const w = await mountDetail({
      ...view(),
      sections: [
        {
          heading: 'Subtasks',
          sectionId: 'subtasks',
          display: 'related',
          isEmpty: false,
          isGrouped: false,
          hasContent: false,
          entities: [{ id: 'TASK-9', type: 'task', title: 'Order catering', hasContent: false }],
        },
      ],
    })
    expect(w.find('.related-rows #TASK-9').text()).toContain('Order catering')
  })

  it('says in the delete confirm that owned entities go too', async () => {
    useSchemaStore().relationTypes.set('subtask', {
      label: 'subtask',
      from: ['task'],
      to: ['task'],
      owning: true,
    })
    confirmMock.mockReset().mockResolvedValue(false)
    const w = await mountDetail(view({ _actions: { delete: true } }))
    await w
      .findAll('button')
      .find((b) => b.text().startsWith('Delete'))!
      .trigger('click')
    expect(confirmMock).toHaveBeenCalledWith(
      expect.objectContaining({
        message: expect.stringContaining('The entities it owns are deleted with it.'),
      })
    )
  })
})
