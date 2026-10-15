import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada, useQueryCache } from '@pinia/colada'
import { defineComponent, reactive } from 'vue'
import SpaceCreateMenu from './SpaceCreateMenu.vue'
import { usePageStore } from '@/stores/pages'
import { useSchemaStore } from '@/stores/schema'
import { useSpaceStore } from '@/stores/space'
import { useUIStore } from '@/stores/ui'
import type { CalendarConfig, Entity, KanbanConfig, SidebarPage } from '@/types'

const createRelationMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  createRelation: (...args: unknown[]) => createRelationMock(...args),
}))

const pushMock = vi.fn()
/* The route on screen; the menu reads which view it shows. */
const route = reactive<{ name: string; params: Record<string, string> }>({
  name: 'dashboard',
  params: {},
})
vi.mock('vue-router', () => ({ useRouter: () => ({ push: pushMock }), useRoute: () => route }))

/* The create dialog, reduced to the two events the menu listens to. */
const ModalStub = defineComponent({
  name: 'InlineCreateFormModal',
  emits: ['created', 'created-another', 'close'],
  template: '<div />',
})

const task = { id: 'TASK-9', type: 'task', properties: {} } as Entity

const topicPage: SidebarPage = {
  label: 'Topic',
  entity_type: 'topic',
  tabs: [
    {
      id: 'board',
      label: 'Board',
      view: 'kanban',
      target: 'tasks',
      scope: 'relation',
      relation: 'contains',
      direction: 'outgoing',
      links: [{ type: 'task', relation: 'contains', direction: 'outgoing' }],
    },
    {
      id: 'timeline',
      label: 'Timeline',
      view: 'gantt',
      target: 'plan',
      scope: 'root',
      links: [{ type: 'task', relation: 'contains', direction: 'outgoing' }],
    },
  ],
}

/* The menu, open: the trigger and every row are rendered. */
const MenuStub = {
  template:
    '<div data-testid="menu"><slot name="trigger" :toggle="() => {}" :attrs="{}" /><slot /></div>',
}

function mountMenu() {
  return mount(SpaceCreateMenu, {
    global: {
      plugins: [pinia, PiniaColada],
      stubs: { InlineCreateFormModal: ModalStub, RlMenu: MenuStub },
    },
  })
}

const rowLabels = (wrapper: ReturnType<typeof mountMenu>) =>
  wrapper.findAll('[role="menuitem"]').map((el) => el.text())

/* Opens the Create menu's Task row; `save` then emits the dialog's result. */
async function openTaskDialog() {
  const wrapper = mountMenu()
  const row = wrapper.findAll('[role="menuitem"]').find((el) => el.text() === 'Task')
  await row!.trigger('click')
  return {
    wrapper,
    async save(entity: Entity, event: 'created' | 'created-another' = 'created') {
      wrapper.findComponent(ModalStub).vm.$emit(event, entity)
      await flushPromises()
    },
  }
}

async function createFromMenu(entity: Entity, event: 'created' | 'created-another' = 'created') {
  await (await openTaskDialog()).save(entity, event)
}

const linkArgs = (anchor: string) => ['topic', anchor, 'contains', task.id, undefined, 'outgoing']

let pinia: ReturnType<typeof createPinia>

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  createRelationMock.mockReset().mockResolvedValue(undefined)
  pushMock.mockReset()
  Object.assign(route, { name: 'dashboard', params: {} })
  useSpaceStore().set({ create: [{ type: 'task', label: 'Task', form: 'create_task' }] })
  usePageStore().set({ topic: topicPage })
})

describe('SpaceCreateMenu on an entity page', () => {
  it.each(['board', 'timeline'])(
    'links a new row to the page entity from the %s tab',
    async (tab) => {
      usePageStore().current = { page: 'topic', tab, entity: 'TOPIC-1', ref: 'TOPIC-1@draft' }
      await createFromMenu(task)
      expect(createRelationMock).toHaveBeenCalledWith(...linkArgs('TOPIC-1@draft'))
      expect(pushMock).toHaveBeenCalledWith(`/entity/task/${task.id}`)
    }
  )

  it('links each row made with add another, and refreshes its lists', async () => {
    usePageStore().current = { page: 'topic', tab: 'board', entity: 'TOPIC-1' }
    const invalidate = vi.spyOn(useQueryCache(), 'invalidateQueries')
    await createFromMenu(task, 'created-another')
    expect(createRelationMock).toHaveBeenCalledWith(...linkArgs('TOPIC-1'))
    expect(invalidate).toHaveBeenCalledWith({ key: ['entities', 'task', 'list'] })
  })

  it('links to the page the dialog was opened on, not the page on screen at save', async () => {
    usePageStore().current = { page: 'topic', tab: 'board', entity: 'TOPIC-1' }
    const dialog = await openTaskDialog()
    usePageStore().current = { page: 'topic', tab: 'board', entity: 'TOPIC-2' }
    await dialog.save(task)
    expect(createRelationMock).toHaveBeenCalledWith(...linkArgs('TOPIC-1'))
    expect(createRelationMock).toHaveBeenCalledTimes(1)
  })

  it('names the row to link by hand when the link fails, and still opens it', async () => {
    usePageStore().current = { page: 'topic', tab: 'board', entity: 'TOPIC-1' }
    createRelationMock.mockRejectedValue(new Error('forbidden'))
    const errorSpy = vi.spyOn(useUIStore(), 'error')
    await createFromMenu(task)
    expect(errorSpy).toHaveBeenCalledWith(expect.stringContaining(`${task.id} was created`))
    expect(pushMock).toHaveBeenCalledWith(`/entity/task/${task.id}`)
  })

  it('links nothing outside an entity page', async () => {
    await createFromMenu(task)
    expect(createRelationMock).not.toHaveBeenCalled()
    expect(pushMock).toHaveBeenCalledWith(`/entity/task/${task.id}`)
  })
})

describe('SpaceCreateMenu rows', () => {
  const rows = [
    { type: 'task', label: 'Task', form: 'create_task' },
    { type: 'note', label: 'Note', form: 'create_note' },
  ]

  beforeEach(() => {
    useSpaceStore().set({ create: rows })
  })

  it('offers every type outside an entity page', () => {
    expect(rowLabels(mountMenu())).toEqual(['Task', 'Note'])
  })

  it('offers only the types it can link on an entity page', () => {
    usePageStore().current = { page: 'topic', tab: 'board', entity: 'TOPIC-1' }
    expect(rowLabels(mountMenu())).toEqual(['Task'])
  })

  it('offers what the open tab can link', async () => {
    const blocks = { type: 'task', relation: 'blocks', direction: 'incoming' as const }
    usePageStore().set({
      topic: {
        ...topicPage,
        tabs: [
          { ...topicPage.tabs[0]! },
          { id: 'blockers', label: 'Blockers', view: 'list', links: [blocks] },
          { id: 'notes', label: 'Notes', view: 'list' },
        ],
      },
    })
    usePageStore().current = { page: 'topic', tab: 'blockers', entity: 'TOPIC-1' }
    const wrapper = mountMenu()
    expect(rowLabels(wrapper)).toEqual(['Task'])
    // Two relations for a task on the page, and none on this tab: no link, so no row.
    usePageStore().current = { page: 'topic', tab: 'notes', entity: 'TOPIC-1' }
    await flushPromises()
    expect(wrapper.find('[data-testid="menu"]').exists()).toBe(false)
  })

  it('keeps the button busy and offers nothing while an entity page opens', () => {
    usePageStore().current = { page: 'topic', tab: 'board', entity: 'TOPIC-1' }
    usePageStore().opening = true
    const wrapper = mountMenu()
    const button = wrapper.find('[data-testid="space-create"]')
    expect(button.attributes('aria-busy')).toBe('true')
    expect(button.attributes('aria-disabled')).toBe('true')
    expect(rowLabels(wrapper)).toEqual([])
  })

  it('does not open from the keyboard while an entity page opens', async () => {
    usePageStore().opening = true
    const wrapper = mount(SpaceCreateMenu, {
      global: { plugins: [pinia, PiniaColada], stubs: { InlineCreateFormModal: ModalStub } },
    })
    const button = wrapper.find('[data-testid="space-create"]')
    await button.trigger('keydown', { key: 'ArrowDown' })
    await button.trigger('click')
    expect(button.attributes('aria-expanded')).toBe('false')
    usePageStore().opening = false
    await flushPromises()
    await button.trigger('keydown', { key: 'ArrowDown' })
    expect(button.attributes('aria-expanded')).toBe('true')
    wrapper.unmount()
  })

  it('hides the button on an entity page that can link none of the types', () => {
    usePageStore().current = { page: 'topic', tab: 'board', entity: 'TOPIC-1' }
    useSpaceStore().set({ create: [rows[1]!] })
    expect(mountMenu().find('[data-testid="space-create"]').exists()).toBe(false)
  })

  it('keeps an open dialog when a navigation empties the menu', async () => {
    usePageStore().current = { page: 'topic', tab: 'board', entity: 'TOPIC-1' }
    const dialog = await openTaskDialog()
    usePageStore().set({ topic: { ...topicPage, tabs: [] } })
    await flushPromises()
    expect(dialog.wrapper.find('[data-testid="menu"]').exists()).toBe(false)
    await dialog.save(task)
    expect(createRelationMock).toHaveBeenCalledWith(...linkArgs('TOPIC-1'))
  })
})

describe('SpaceCreateMenu order', () => {
  const rows = [
    { type: 'deal', label: 'Deal', form: 'create_deal' },
    { type: 'org', label: 'Organisation', form: 'create_org' },
    { type: 'person', label: 'Person', form: 'create_person' },
  ]

  const pipeline: KanbanConfig = {
    entity: 'deal',
    column_property: 'stage',
    card: { title: 'title' },
  }
  const agenda: CalendarConfig = {
    default_view: 'month',
    week_start: 'monday',
    day_start: '08:00',
    day_end: '18:00',
    max_events_per_day: 3,
    sources: [
      { entity: 'person', date: 'birthday', max_span: 0 },
      { entity: 'org', date: 'founded', max_span: 0 },
    ],
  }

  /* The menu's children in order: a row's label, or `|` for the separator. */
  const items = (wrapper: ReturnType<typeof mountMenu>) =>
    wrapper
      .findAll('[role="menuitem"], [role="separator"]')
      .map((el) => (el.attributes('role') === 'separator' ? '|' : el.text()))

  beforeEach(() => {
    useSpaceStore().set({ create: rows })
    const schema = useSchemaStore()
    schema.lists = new Map([['orgs', { entity: 'org', columns: [] }]])
    schema.kanbans = new Map([['pipeline', pipeline]])
    schema.calendars = new Map([['agenda', agenda]])
  })

  it('keeps config order on a page without view types', () => {
    expect(items(mountMenu())).toEqual(['Deal', 'Organisation', 'Person'])
  })

  it("puts a list's type first", () => {
    Object.assign(route, { name: 'list', params: { id: 'orgs' } })
    expect(items(mountMenu())).toEqual(['Organisation', '|', 'Deal', 'Person'])
  })

  it("puts a board's type first, with no separator when nothing follows", () => {
    Object.assign(route, { name: 'kanban', params: { id: 'pipeline' } })
    expect(items(mountMenu())).toEqual(['Deal', '|', 'Organisation', 'Person'])
    useSpaceStore().set({ create: [rows[0]!] })
    expect(items(mountMenu())).toEqual(['Deal'])
  })

  it("leads with a calendar's types in config order", () => {
    Object.assign(route, { name: 'calendar', params: { id: 'agenda' } })
    expect(items(mountMenu())).toEqual(['Organisation', 'Person', '|', 'Deal'])
  })

  it('follows the view of the open page tab', async () => {
    usePageStore().set({
      crm: {
        label: 'CRM',
        tabs: [
          { id: 'orgs', label: 'Orgs', view: 'list', target: 'orgs' },
          { id: 'pipeline', label: 'Pipeline', view: 'kanban', target: 'pipeline' },
        ],
      },
    })
    Object.assign(route, { name: 'page', params: { page: 'crm', tab: 'orgs' } })
    const wrapper = mountMenu()
    expect(items(wrapper)).toEqual(['Organisation', '|', 'Deal', 'Person'])
    route.params = { page: 'crm', tab: 'pipeline' }
    await flushPromises()
    expect(items(wrapper)).toEqual(['Deal', '|', 'Organisation', 'Person'])
  })

  it("keeps config order for a view type the space doesn't create", () => {
    useSchemaStore().lists = new Map([['notes', { entity: 'note', columns: [] }]])
    Object.assign(route, { name: 'list', params: { id: 'notes' } })
    expect(items(mountMenu())).toEqual(['Deal', 'Organisation', 'Person'])
  })
})
