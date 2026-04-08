import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { useEntitiesStore } from '@/stores/entities'
import { _resetModalStack } from '@/composables/modalStack'
import type { Entity } from '@/types'

// Router stubs — EntityList reads both `useRouter` (for open/edit/create
// navigation) and `useRoute` (for URL-filter sync). The delete flow we are
// testing doesn't push routes, so minimal stubs suffice.
const routerPush = vi.fn()
const routerReplace = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush, replace: routerReplace }),
  useRoute: () => ({ query: {}, path: '/list/tickets-list', name: 'list' }),
}))

// Integration test for the delete-flow wiring:
//   onDelete (from useListKeyboard) → pendingDelete → ConfirmModal render
//   → confirmDelete → entitiesStore.remove
// This is the seam the reviewer flagged: unit tests for useListKeyboard and
// ConfirmModal individually do not prove that EntityList assembles them
// correctly.

describe('EntityList delete integration', () => {
  const listId = 'tickets-list'
  const entityType = 'ticket'

  function makeEntity(id: string): Entity {
    return {
      id,
      type: entityType,
      properties: { title: `Title ${id}` },
    }
  }

  function seedSchema() {
    const schemaStore = useSchemaStore()
    // Minimal list config: one text column, no filters, default page size.
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Tickets',
      entity: entityType,
      columns: [{ property: 'title', label: 'Title' }],
    } as never)
    // Minimal entity type so entityType computed resolves.
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Ticket',
      properties: {
        title: { type: 'string', values: null },
      },
    } as never)
  }

  function seedEntities(entities: Entity[]) {
    const entitiesStore = useEntitiesStore()
    entitiesStore.fetchList = vi.fn().mockResolvedValue({
      data: entities,
      meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
      included: {},
    })
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    _resetModalStack()
    routerPush.mockClear()
  })

  afterEach(() => {
    document.body.innerHTML = ''
    _resetModalStack()
  })

  async function mountList(entities: Entity[]) {
    seedSchema()
    seedEntities(entities)
    const wrapper = mount(EntityList, {
      props: { listId },
      attachTo: document.body,
    })
    await flushPromises()
    return wrapper
  }

  function overlay(): HTMLElement | null {
    return document.querySelector<HTMLElement>('.modal-overlay')
  }

  function modalButtons(): HTMLButtonElement[] {
    return Array.from(
      document.querySelectorAll<HTMLButtonElement>('.modal-actions button')
    )
  }

  it('does not show delete modal by default', async () => {
    const wrapper = await mountList([makeEntity('T-1'), makeEntity('T-2')])
    expect(overlay()).toBeNull()
    wrapper.unmount()
  })

  it('clicking delete button opens confirm modal for that entity', async () => {
    const entities = [makeEntity('T-1'), makeEntity('T-2')]
    const wrapper = await mountList(entities)

    const deleteButtons = wrapper.findAll('.delete-btn')
    expect(deleteButtons).toHaveLength(2)
    await deleteButtons[1].trigger('click')
    await flushPromises()

    expect(overlay()).not.toBeNull()
    // The modal's slot references the pending entity id.
    expect(overlay()?.textContent).toContain(entities[1].id)
    wrapper.unmount()
  })

  it('Delete keydown on selected row opens confirm modal for that row', async () => {
    const entities = [makeEntity('T-1'), makeEntity('T-2')]
    const wrapper = await mountList(entities)

    // Select first row via j, then open delete modal via Delete.
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'j', bubbles: true }))
    document.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Delete', bubbles: true })
    )
    await flushPromises()

    expect(overlay()).not.toBeNull()
    expect(overlay()?.textContent).toContain(entities[0].id)
    wrapper.unmount()
  })

  it('Backspace on selected row opens confirm modal', async () => {
    const entities = [makeEntity('T-1')]
    const wrapper = await mountList(entities)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'j', bubbles: true }))
    document.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Backspace', bubbles: true })
    )
    await flushPromises()

    expect(overlay()).not.toBeNull()
    wrapper.unmount()
  })

  it('Cancel button closes modal without deleting', async () => {
    const entities = [makeEntity('T-1')]
    const wrapper = await mountList(entities)
    const entitiesStore = useEntitiesStore()
    const removeSpy = vi.fn().mockResolvedValue(undefined)
    entitiesStore.remove = removeSpy

    await wrapper.find('.delete-btn').trigger('click')
    await flushPromises()

    modalButtons()[0].click()
    await flushPromises()

    expect(overlay()).toBeNull()
    expect(removeSpy).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('Confirm button calls entitiesStore.remove with the pending entity', async () => {
    const entities = [makeEntity('T-1'), makeEntity('T-2')]
    const wrapper = await mountList(entities)
    const entitiesStore = useEntitiesStore()
    const removeSpy = vi.fn().mockResolvedValue(undefined)
    entitiesStore.remove = removeSpy

    // Click delete on the SECOND row — the spy should receive that entity.
    const deleteButtons = wrapper.findAll('.delete-btn')
    await deleteButtons[1].trigger('click')
    await flushPromises()

    modalButtons()[1].click()
    await flushPromises()

    expect(removeSpy).toHaveBeenCalledTimes(1)
    expect(removeSpy).toHaveBeenCalledWith(entities[1].type, entities[1].id)
    wrapper.unmount()
  })

  it('keeps modal open on error and clears busy state', async () => {
    const entities = [makeEntity('T-1')]
    const wrapper = await mountList(entities)
    const entitiesStore = useEntitiesStore()
    entitiesStore.remove = vi.fn().mockRejectedValue(new Error('boom'))
    // The component logs the error via console.error; silence it in the test
    // so the output stays clean.
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

    await wrapper.find('.delete-btn').trigger('click')
    await flushPromises()

    modalButtons()[1].click()
    await flushPromises()

    // Modal stays open so the user can retry or cancel with context.
    expect(overlay()).not.toBeNull()
    // Confirm button is re-enabled after the failure.
    expect(modalButtons()[1].disabled).toBe(false)
    consoleSpy.mockRestore()
    wrapper.unmount()
  })
})
