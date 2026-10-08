import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { usePageStore } from '@/stores/pages'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse, RelationOrder } from '@/types'

// An entity-page tab over a relation orderable on its outgoing side reads in
// relation order, so it leaves the list's default_sort out of the read. These
// tests cover that choice and its fallback.

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/list/tasks', name: 'list' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

const listId = 'tasks'
const scope = { page: 'project', tab: 'tasks', entity: 'PRJ-1' }
const order: RelationOrder = { relation: 'has-task', anchor: 'PRJ-1', anchor_type: 'project', movable: true }

function seed() {
  const schema = useSchemaStore()
  schema.lists.set(listId, {
    id: listId,
    title: 'Tasks',
    entity: 'task',
    columns: [{ property: 'title', label: 'Title' }],
    default_sort: [{ property: 'title', direction: 'asc' }],
  } as never)
  schema.entityTypes.set('task', {
    name: 'task',
    label: 'Task',
    properties: { title: { type: 'string', values: null } },
  } as never)
  schema.relationTypes.set('has-task', {
    name: 'has-task',
    from: ['project'],
    to: ['task'],
    orderable: { outgoing: true, incoming: false },
  } as never)
  usePageStore().set({
    project: {
      label: 'Project',
      entity_type: 'project',
      tabs: [{
        id: 'tasks', label: 'Tasks', view: 'list', target: listId,
        scope: 'relation', relation: 'has-task', direction: 'outgoing',
        links: [{ type: 'task', relation: 'has-task', direction: 'outgoing' }],
      }],
    },
  } as never)
}

function respond(relationOrder?: RelationOrder) {
  const data: Entity[] = [{ id: 'T-1', type: 'task', properties: { title: 'One' } }]
  listEntitiesMock.mockResolvedValue({
    data,
    meta: { total: 1, page: 1, per_page: 25, has_more: false, relation_order: relationOrder },
    included: {},
  } as ListResponse<Entity>)
}

describe('EntityList on a relation-ordered tab', () => {
  let pinia: ReturnType<typeof createPinia>
  const mounted: VueWrapper[] = []
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest('task', 'tasks')
    listEntitiesMock.mockReset()
    seed()
  })
  afterEach(() => {
    for (const w of mounted.splice(0)) w.unmount()
  })

  async function mountList() {
    const wrapper = mount(EntityList, {
      props: { listId, pageScope: scope },
      global: { plugins: [pinia, PiniaColada], stubs: { InlineCreateFormModal: true } },
    })
    mounted.push(wrapper)
    await flushPromises()
    return wrapper
  }

  const sorts = () => listEntitiesMock.mock.calls.map(([, params]) => params.sort)

  it('reads without the default sort and keeps it out', async () => {
    respond(order)
    await mountList()
    expect(sorts()).toEqual([undefined])
  })

  it('falls back to the default sort when the server withholds the order', async () => {
    respond(undefined)
    await mountList()
    expect(sorts()).toEqual([undefined, 'title'])
  })
})
