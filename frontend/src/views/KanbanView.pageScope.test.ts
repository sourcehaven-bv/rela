import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { defineComponent } from 'vue'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { usePageStore } from '@/stores/pages'
import { _setEntityPluralForTest } from '@/api/entities'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import type { Entity, ListResponse } from '@/types'

// A board shown as a tab of an entity page (`pages.<p>.entity_type`) reads
// only the anchor's cards, and a card created there is linked to the anchor
// so it lands on the board it was added from.

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: vi.fn().mockResolvedValue(undefined),
}))

const createRelationMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  createRelation: (...args: unknown[]) => createRelationMock(...args),
}))

const replaceMock = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: replaceMock }),
  useRoute: () => ({ query: {}, path: '/p/topic/TOP-1/board' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

vi.mock('@/composables/useBackTarget', () => ({ useBackTarget: () => null }))

// The dialog itself is covered elsewhere; here it only has to announce a create.
const ModalStub = defineComponent({
  name: 'InlineCreateFormModal',
  emits: ['created', 'created-another', 'close'],
  template: '<div class="modal-stub" />',
})

const scope = { page: 'topic', tab: 'board', entity: 'TOP-1' }

function seed() {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set('taken_bord', {
    entity: 'taak',
    title: 'Taken',
    column_property: 'status',
    columns: [{ value: 'todo', label: 'Te doen' }],
    card: { title: 'title' },
    create_form: 'taak_nieuw',
  } as never)
  schemaStore.entityTypes.set('taak', {
    label: 'Taak',
    properties: { title: { type: 'string' }, status: { type: 'enum', values: ['todo'] } },
  } as never)
  usePageStore().set({
    topic: {
      label: 'Topic',
      entity_type: 'topic',
      tabs: [{
        id: 'board', label: 'Board', view: 'kanban', target: 'taken_bord',
        scope: 'relation', relation: 'bestaat_uit', direction: 'outgoing',
      }],
    },
  })
  listAllEntitiesMock.mockResolvedValue({
    data: [],
    meta: { total: 0, page: 1, per_page: 100, has_more: false },
    included: {},
    _actions: { create: true },
  } as unknown as ListResponse<Entity>)
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest('taak', 'taken')
  _setEntityPluralForTest('topic', 'topics')
  listAllEntitiesMock.mockReset()
  createRelationMock.mockReset()
  replaceMock.mockReset()
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountBoard() {
  seed()
  const wrapper = mount(withPageHeader(KanbanView), {
    attrs: { id: 'taken_bord', pageScope: scope },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada], stubs: { InlineCreateFormModal: ModalStub } },
  })
  await flushPromises()
  return wrapper
}

describe('KanbanView in an entity-page tab', () => {
  it('reads only the anchor’s cards', async () => {
    await mountBoard()
    const params = listAllEntitiesMock.mock.calls[0][1]
    expect(params).toMatchObject({ scope_page: 'topic', scope_tab: 'board', anchor: 'TOP-1' })
  })

  it('links a created card to the anchor before refreshing', async () => {
    const wrapper = await mountBoard()
    const reads = listAllEntitiesMock.mock.calls.length
    createRelationMock.mockImplementation(() => {
      // The link lands before the board reads again, so the card shows.
      expect(listAllEntitiesMock.mock.calls.length).toBe(reads)
      return Promise.resolve()
    })

    const newButton = wrapper.findAll('button').find((b) => b.text() === 'New')
    expect(newButton).toBeDefined()
    await newButton?.trigger('click')
    wrapper.findComponent(ModalStub).vm.$emit('created', { id: 'TAAK-9', type: 'taak', properties: {} })
    await flushPromises()

    expect(createRelationMock).toHaveBeenCalledWith('topic', 'TOP-1', 'bestaat_uit', 'TAAK-9', undefined, 'outgoing')
    expect(listAllEntitiesMock.mock.calls.length).toBeGreaterThan(reads)
  })
})
