import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada, useQueryCache } from '@pinia/colada'
import { defineComponent } from 'vue'
import SpaceCreateMenu from './SpaceCreateMenu.vue'
import { usePageStore } from '@/stores/pages'
import { useSpaceStore } from '@/stores/space'
import { useUIStore } from '@/stores/ui'
import type { Entity, SidebarPage } from '@/types'

const createRelationMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  createRelation: (...args: unknown[]) => createRelationMock(...args),
}))

const pushMock = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push: pushMock }) }))

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
