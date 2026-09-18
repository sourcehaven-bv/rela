// Create-related-entity from the detail page (TKT-R4BMJM).
//
// The interesting property here is a NEGATIVE one, and it cost a real bug to
// learn: this host must NOT create the relation itself.
//
// The pre-link travels to the embedded form as a prop, and that form owns both
// directions — `linkAs: 'to'` rides its create payload, `linkAs: 'from'` is its
// own post-create call. Linking here as well created the same edge twice, and
// the second attempt failed with "relation already exists", reporting a link
// failure for an edge that was already correct. Manual verification caught it;
// no unit test did, which is why this file exists.
//
// Per ruling 10: the central assertion is an absence, so every test pairs it
// with a positive assertion that the page rendered and the flow ran.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import { useSchemaStore } from '@/stores/schema'
import type { Entity } from '@/types'
import type { ViewResponse, ViewSection, ViewSectionCreate } from '@/api'

const fetchViewMock = vi.fn()
const getCommandsMock = vi.fn()
// The relation WRITE. Asserting on this is the point: it answers whether a
// second edge-create left the client at all.
const createRelationMock = vi.fn()

vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: (...a: unknown[]) => getCommandsMock(...a),
  createRelation: (...a: unknown[]) => createRelationMock(...a),
}))
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  createRelation: (...a: unknown[]) => createRelationMock(...a),
}))
vi.mock('vue-router', async (orig) => ({
  ...(await orig<typeof import('vue-router')>()),
  useRoute: () => ({ query: {}, params: {}, path: '/entity/category/backend', fullPath: '/entity/category/backend' }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}))

const entityType = 'category'
const entityId = 'backend'

/** The affordance the server sends for an INCOMING section. */
function createAffordance(over: Partial<ViewSectionCreate> = {}): ViewSectionCreate {
  return {
    relation: 'belongs-to',
    // The new ticket is the FROM of `belongs-to` — the direction that needs a
    // post-create call, and the one the double-link bug lived on.
    linkAs: 'from',
    peerId: entityId,
    flow: 'modal',
    targets: [{ entityType: 'ticket', formId: 'create_ticket', label: 'Ticket' }],
    ...over,
  }
}

function viewWithCreateSection(create?: ViewSectionCreate): ViewResponse {
  const sections: ViewSection[] = [
    {
      heading: 'Tickets',
      sectionId: 'tickets',
      display: 'cards',
      isEmpty: true,
      isGrouped: false,
      hasContent: false,
      create,
    },
  ]
  return {
    entry: {
      id: entityId,
      type: entityType,
      properties: { name: 'Backend' },
      _actions: { update: true, delete: true },
    } as Entity,
    sections,
  }
}

/** Stands in for the create modal, so a test can fire `created` directly. */
const modalStub = {
  name: 'InlineCreateFormModal',
  props: ['show', 'formId', 'entityType', 'template', 'link', 'world'],
  emits: ['close', 'created'],
  template: '<div class="modal-stub" />',
}

describe('EntityDetail create affordance', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()
    getCommandsMock.mockResolvedValue([])
    createRelationMock.mockResolvedValue(undefined)
    const schema = useSchemaStore()
    schema.entityTypes.set(entityType, { name: entityType, label: 'Category' } as never)
    schema.entityTypes.set('ticket', { name: 'ticket', label: 'Ticket' } as never)
    schema.loaded = true
  })

  async function mountDetail(view: ViewResponse) {
    fetchViewMock.mockResolvedValue(view)
    const wrapper = mount(EntityDetail, {
      props: { entityType, entityId },
      attachTo: document.body,
      global: {
        plugins: [pinia, PiniaColada],
        stubs: { InlineCreateFormModal: modalStub, DocumentsPanel: true, CommentsPanel: true },
      },
    })
    await flushPromises()
    return wrapper
  }

  it('renders the button only for a section the server opted in', async () => {
    const withCreate = await mountDetail(viewWithCreateSection(createAffordance()))
    expect(withCreate.find('button.btn-section-create').exists()).toBe(true)

    // The paired positive/negative: same mount shape, affordance absent.
    const without = await mountDetail(viewWithCreateSection(undefined))
    expect(without.find('.view-section').exists()).toBe(true) // did render
    expect(without.find('button.btn-section-create').exists()).toBe(false)
  })

  it('opens the modal with the pre-link as a PROP, not a navigation', async () => {
    const wrapper = await mountDetail(viewWithCreateSection(createAffordance()))

    await wrapper.find('button.btn-section-create').trigger('click')
    await flushPromises()

    const modal = wrapper.findComponent(modalStub)
    expect(modal.exists()).toBe(true)
    // The prop channel exists because an embedded form reads an EMPTY query:
    // it mounts over this page, so honouring the URL would pre-fill from here.
    expect(modal.props('link')).toEqual({
      relation: 'belongs-to',
      peer: entityId,
      linkAs: 'from',
    })
    expect(modal.props('formId')).toBe('create_ticket')
  })

  it('does NOT create the relation itself — the form owns both directions', async () => {
    // The regression. `linkAs: 'from'` is precisely the case this host used to
    // "help" with, producing a duplicate edge and a false failure toast.
    const wrapper = await mountDetail(viewWithCreateSection(createAffordance()))

    await wrapper.find('button.btn-section-create').trigger('click')
    await flushPromises()

    const created: Entity = { id: 'TKT-009', type: 'ticket', properties: {} }
    wrapper.findComponent(modalStub).vm.$emit('created', created)
    await flushPromises()

    expect(createRelationMock).not.toHaveBeenCalled()
    // Anti-vacuity: the flow really ran — the modal closed and the view
    // refetched, which is this handler's actual job.
    expect(wrapper.findComponent(modalStub).exists()).toBe(false)
    expect(fetchViewMock.mock.calls.length).toBeGreaterThan(1)
  })

  it('refetches the whole view after a create', async () => {
    // "In place" means "no navigation", not a partial update: loadView replaces
    // viewData wholesale because there is no per-section fetch.
    const wrapper = await mountDetail(viewWithCreateSection(createAffordance()))
    const before = fetchViewMock.mock.calls.length

    await wrapper.find('button.btn-section-create').trigger('click')
    await flushPromises()
    wrapper.findComponent(modalStub).vm.$emit('created', {
      id: 'TKT-010',
      type: 'ticket',
      properties: {},
    } as Entity)
    await flushPromises()

    expect(fetchViewMock.mock.calls.length).toBe(before + 1)
  })
})
