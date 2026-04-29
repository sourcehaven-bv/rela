import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { useEntitiesStore } from '@/stores/entities'
import { _resetModalStack } from '@/composables/modalStack'
import type { Entity } from '@/types'

// Router stubs — EntityList reads both `useRouter` (for open/edit/create
// navigation) and `useRoute` (for URL-filter sync). The delete flow uses a
// minimal stub; the search-flow tests below mutate `mockRoute.query` to
// simulate `?q=` deep-links and assert that the composable reflects them.
const routerPush = vi.fn()
const routerReplace = vi.fn()
const mockRoute = { query: {} as Record<string, string>, path: '/list/tickets-list', name: 'list' }
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush, replace: routerReplace }),
  useRoute: () => mockRoute,
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
    routerReplace.mockClear()
    mockRoute.query = {}
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

describe('EntityList search integration', () => {
  const listId = 'tickets-list'
  const entityType = 'ticket'

  function seedSchema() {
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Tickets',
      entity: entityType,
      columns: [{ property: 'title', label: 'Title' }],
    } as never)
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Ticket',
      properties: { title: { type: 'string', values: null } },
    } as never)
  }

  function fakeFetchList() {
    const entitiesStore = useEntitiesStore()
    const fetchList = vi.fn().mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 25, has_more: false },
      included: {},
    })
    entitiesStore.fetchList = fetchList
    return fetchList
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    _resetModalStack()
    routerPush.mockClear()
    routerReplace.mockClear()
    mockRoute.query = {}
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    document.body.innerHTML = ''
    _resetModalStack()
  })

  it('AC3: hydrates the search box from ?q= in the URL', async () => {
    seedSchema()
    fakeFetchList()
    mockRoute.query = { q: 'foo' }

    const wrapper = mount(EntityList, { props: { listId }, attachTo: document.body })
    await flushPromises()

    const input = wrapper.find<HTMLInputElement>('.search-box input[type="search"]')
    expect(input.element.value).toBe('foo')
    wrapper.unmount()
  })

  it('AC2: typing fires exactly one fetch after the debounce window', async () => {
    seedSchema()
    const fetchList = fakeFetchList()
    const wrapper = mount(EntityList, { props: { listId }, attachTo: document.body })
    await flushPromises()

    // Initial mount fetch already happened.
    const initialCallCount = fetchList.mock.calls.length

    const input = wrapper.find<HTMLInputElement>('.search-box input[type="search"]')
    // Type three characters in quick succession.
    for (const ch of ['T', 'TK', 'TKT']) {
      input.element.value = ch
      await input.trigger('input')
    }

    // No fetch yet — still inside the debounce window.
    expect(fetchList.mock.calls.length).toBe(initialCallCount)

    // Advance past the debounce.
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()

    // Exactly one extra fetch fired, with q="TKT" in the params.
    expect(fetchList.mock.calls.length).toBe(initialCallCount + 1)
    const lastCall = fetchList.mock.calls[fetchList.mock.calls.length - 1]
    expect(lastCall[1]).toMatchObject({ q: 'TKT' })

    wrapper.unmount()
  })

  it('AC4: clearing the search restores the unfiltered list and removes q from URL', async () => {
    seedSchema()
    const fetchList = fakeFetchList()
    mockRoute.query = { q: 'foo' }

    const wrapper = mount(EntityList, { props: { listId }, attachTo: document.body })
    await flushPromises()

    // Sanity: initial fetch carries q=foo.
    const before = fetchList.mock.calls[fetchList.mock.calls.length - 1]
    expect(before[1]).toMatchObject({ q: 'foo' })

    // Click the SearchBox clear button.
    await wrapper.find('.search-box .clear-btn').trigger('click')
    await flushPromises()

    // Router was asked to drop `q` from the query.
    expect(routerReplace).toHaveBeenCalled()
    const lastReplace = routerReplace.mock.calls[routerReplace.mock.calls.length - 1][0]
    expect(lastReplace.query.q).toBeUndefined()

    // Subsequent fetch goes out without q.
    await flushPromises()
    const after = fetchList.mock.calls[fetchList.mock.calls.length - 1]
    expect(after[1].q).toBeUndefined()

    wrapper.unmount()
  })
})
