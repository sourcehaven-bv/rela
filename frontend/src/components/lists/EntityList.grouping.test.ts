import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import InlineCreateFormModal from '@/components/forms/InlineCreateFormModal.vue'
import { useSchemaStore } from '@/stores/schema'
import { usePageStore } from '@/stores/pages'
import { _setEntityPluralForTest } from '@/api/entities'
import { collapsedStorageKey } from '@/composables/useListGrouping'
import type { Entity, ListResponse } from '@/types'

// A list with `group_by:` loads its rows in one piece and splits them into
// sections. These tests cover what only the list page does with the sections:
// which read it makes, the Add control's prefill, and the remembered collapse.
// The splitting itself is covered in useListGrouping.test.ts.

const listEntitiesMock = vi.fn()
const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
}))

const createRelationMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  createRelation: (...args: unknown[]) => createRelationMock(...args),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/list/tasks', name: 'list' }),
  RouterLink: {
    props: ['to'],
    template: '<a><slot /></a>',
  },
}))

describe('EntityList grouped by a property', () => {
  const listId = 'tasks'
  const entityType = 'task'

  function seedSchema(groupBy: unknown) {
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Tasks',
      entity: entityType,
      create_form: 'task_form',
      columns: [
        { property: 'title', label: 'Title' },
        { property: 'status', label: 'Status' },
      ],
      default_sort: [{ property: 'title', direction: 'asc' }],
      group_by: groupBy,
    } as never)
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Task',
      properties: {
        title: { type: 'string', values: null },
        status: { type: 'enum', values: ['todo', 'doing', 'done'], labels: { todo: 'To do' } },
      },
    } as never)
  }

  function task(id: string, title: string, status?: string): Entity {
    return { id, type: entityType, properties: { title, ...(status ? { status } : {}) } }
  }

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(entityType, 'tasks')
    listEntitiesMock.mockReset()
    listAllEntitiesMock.mockReset()
    createRelationMock.mockReset()
    localStorage.clear()
  })

  const mounted: VueWrapper[] = []
  afterEach(() => {
    for (const w of mounted.splice(0)) w.unmount()
  })

  async function mountList(
    response: Partial<ListResponse<Entity>['meta']> = {},
    pageScope?: { page: string; tab: string; entity: string },
  ) {
    seedSchema({
      property: 'status',
      groups: [{ value: 'doing', label: 'In progress' }],
      max_rows: 300,
    })
    const data = [
      task('T-1', 'Write docs', 'todo'),
      task('T-2', 'Ship it', 'done'),
      task('T-3', 'Triage'),
    ]
    listAllEntitiesMock.mockResolvedValue({
      data,
      meta: { total: data.length, page: 1, per_page: data.length, has_more: false, ...response },
      included: {},
    } as ListResponse<Entity>)
    const wrapper = mount(EntityList, {
      props: { listId, pageScope },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada], stubs: { InlineCreateFormModal: true } },
    })
    mounted.push(wrapper)
    await flushPromises()
    return wrapper
  }

  function sectionTitles(wrapper: VueWrapper) {
    // The disclosure is named "Collapse <title>" while the section is open.
    return wrapper
      .findAll('.rl-table-section__header .rl-disclosure')
      .map((d) => d.attributes('aria-label')?.replace(/^Collapse /, ''))
  }

  it('loads every row up to max_rows, sorted by the group property first', async () => {
    const wrapper = await mountList()
    expect(listEntitiesMock).not.toHaveBeenCalled()
    expect(listAllEntitiesMock).toHaveBeenCalledTimes(1)
    const [type, params, , options] = listAllEntitiesMock.mock.calls[0]
    expect(type).toBe(entityType)
    expect(params.sort).toBe('status,title')
    expect(params.page).toBeUndefined()
    expect(options).toEqual({ maxRows: 300 })
    expect(wrapper.text()).toContain('Write docs')
  })

  it('shows a section per enum value in order, then (none), and no pager', async () => {
    const wrapper = await mountList()
    expect(sectionTitles(wrapper)).toEqual(['To do', 'In progress', 'done', '(none)'])
    expect(wrapper.find('[data-testid="group-truncated"]').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'Pagination' }).exists()).toBe(false)
  })

  it('leaves out the column the list is grouped by', async () => {
    const wrapper = await mountList()
    expect(wrapper.text()).toContain('Title')
    expect(wrapper.text()).not.toContain('Status')
  })

  it("prefills the section's value when a row is added from it", async () => {
    const wrapper = await mountList()
    await wrapper.find('button[aria-label="Add to In progress"]').trigger('click')
    const modal = wrapper.findComponent(InlineCreateFormModal)
    expect(modal.exists()).toBe(true)
    expect(modal.props('prefill')).toEqual({ properties: { status: 'doing' } })
    expect(modal.props('addAnother')).toBe(false)
  })

  it('remembers a closed section for the next visit', async () => {
    const wrapper = await mountList()
    const done = wrapper.findAll('.rl-table-section__header .rl-disclosure')[2]
    await done.trigger('click')
    expect(JSON.parse(localStorage.getItem(collapsedStorageKey(listId))!)).toEqual(['value:done'])
    expect(wrapper.text()).not.toContain('Ship it')
  })

  it('says so when the list holds more rows than it loaded', async () => {
    const wrapper = await mountList({ total: 900, has_more: true })
    expect(wrapper.find('[data-testid="group-truncated"]').text()).toContain('first 3 of 900')
  })

  // In a tab of an entity page the whole-list read is still the tab's read:
  // only the anchor's rows, and a row added to a section joins the anchor too.
  describe('in an entity-page tab', () => {
    const scope = { page: 'topic', tab: 'tabel', entity: 'TOP-1' }
    beforeEach(() => {
      usePageStore().set({
        topic: {
          label: 'Topic',
          entity_type: 'topic',
          tabs: [{
            id: 'tabel', label: 'Tabel', view: 'list', target: listId,
            scope: 'relation', relation: 'bestaat_uit', direction: 'outgoing',
          }],
        },
      })
    })

    it('reads only the anchor’s rows', async () => {
      await mountList({}, scope)
      const [, params] = listAllEntitiesMock.mock.calls[0]
      expect(params).toMatchObject({ scope_page: 'topic', scope_tab: 'tabel', anchor: 'TOP-1', sort: 'status,title' })
    })

    it('links a row added from a section to the anchor', async () => {
      createRelationMock.mockResolvedValue(undefined)
      const wrapper = await mountList({}, scope)
      await wrapper.find('button[aria-label="Add to In progress"]').trigger('click')
      wrapper.findComponent(InlineCreateFormModal).vm.$emit('created', task('T-9', 'New', 'doing'))
      await flushPromises()
      expect(createRelationMock).toHaveBeenCalledWith('topic', 'TOP-1', 'bestaat_uit', 'T-9', undefined, 'outgoing')
    })
  })
})
