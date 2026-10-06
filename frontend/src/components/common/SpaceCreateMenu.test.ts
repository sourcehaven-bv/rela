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

/* Opens the Create menu's Task row; `save` then emits the dialog's result. */
async function openTaskDialog() {
  const wrapper = mount(SpaceCreateMenu, {
    global: {
      plugins: [pinia, PiniaColada],
      stubs: { InlineCreateFormModal: ModalStub, RlMenu: { template: '<div><slot /></div>' } },
    },
  })
  const row = wrapper.findAll('[role="menuitem"], button').find((el) => el.text() === 'Task')
  await row!.trigger('click')
  return {
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

  it('links nothing for a type the page has no link for', async () => {
    usePageStore().current = { page: 'topic', tab: 'board', entity: 'TOPIC-1' }
    useSpaceStore().set({ create: [{ type: 'note', label: 'Task', form: 'create_note' }] })
    await createFromMenu({ ...task, id: 'NOTE-1', type: 'note' })
    expect(createRelationMock).not.toHaveBeenCalled()
  })
})
