import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import Pagination from './Pagination.vue'
import ConfirmModal from '@/components/ui/ConfirmModal.vue'
import { useSchemaStore } from '@/stores/schema'
import { useToasts } from 'rela-components/components/feedback/useToasts'
import { ApiError } from '@/api'
import { _setEntityPluralForTest } from '@/api/entities'
import { _resetModalStack } from '@/composables/modalStack'
import { useConfirmHost, _resetConfirmForTest } from '@/composables/useConfirm'
import type { Entity, ListResponse } from '@/types'

/**
 * A cell's value without its stacked-row label.
 *
 * Every cell carries `rl-table-row__cell-label` for the stacked layout. It is
 * aria-hidden and `display: none` on a wide screen, but `.text()` does not
 * care about CSS, so it has to be subtracted explicitly.
 */
function cellText(cell: {
  text: () => string
  find: (s: string) => { exists: () => boolean; text: () => string }
}): string {
  const label = cell.find('.rl-table-row__cell-label')
  const full = cell.text()
  return label.exists() ? full.slice(label.text().length).trim() : full
}

// EntityList fetches via the api layer (useQuery) and deletes and restores
// via it, so mock the api functions, not the entities store.
const listEntitiesMock = vi.fn()
const deleteEntityMock = vi.fn()
const restoreEntityMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
  deleteEntity: (...args: unknown[]) => deleteEntityMock(...args),
  restoreEntity: (...args: unknown[]) => restoreEntityMock(...args),
}))

// Router stubs — EntityList reads both `useRouter` (for open/edit/create
// navigation) and `useRoute` (for URL-filter sync). The delete flow uses a
// minimal stub; the search-flow tests below mutate `mockRoute.query` to
// simulate `?q=` deep-links and assert that the composable reflects them.
const routerPush = vi.fn()
const routerReplace = vi.fn()
const mockRoute = { query: {} as Record<string, string>, path: '/list/tickets-list', name: 'list' }
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: routerPush, replace: routerReplace }),
  useRoute: () => mockRoute,
}))

// Integration test for bulk delete: select rows, delete them from the bulk
// bar or with Delete/Backspace, no confirm, and a toast offering Undo.
// Mounted alongside a useConfirmHost-bound ConfirmModal, as App.vue wires it,
// so the tests can assert that no confirm opens.

describe('EntityList bulk delete', () => {
  const listId = 'tickets-list'
  const entityType = 'ticket'

  function makeEntity(id: string, actions?: Record<string, boolean>): Entity {
    return {
      id,
      type: entityType,
      properties: { title: `Title ${id}` },
      ...(actions ? { _actions: actions } : {}),
    }
  }

  function seedSchema() {
    const schemaStore = useSchemaStore()
    // Minimal list config: one text column, no filters, no bulk actions.
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Tickets',
      entity: entityType,
      columns: [{ property: 'title', label: 'Title' }],
    } as never)
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'ticket',
      properties: {
        title: { type: 'string', values: null },
      },
    } as never)
  }

  function seedEntities(entities: Entity[]): ListResponse<Entity> {
    const response: ListResponse<Entity> = {
      data: entities,
      meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
      included: {},
    }
    listEntitiesMock.mockResolvedValue(response)
    return response
  }

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(entityType, 'tickets')
    listEntitiesMock.mockReset()
    deleteEntityMock.mockReset().mockResolvedValue(undefined)
    restoreEntityMock.mockReset().mockResolvedValue(undefined)
    _resetModalStack()
    _resetConfirmForTest()
    routerPush.mockClear()
    routerReplace.mockClear()
    mockRoute.query = {}
  })

  afterEach(() => {
    document.body.innerHTML = ''
    _resetModalStack()
    _resetConfirmForTest()
  })

  const Host = defineComponent({
    props: { listId: { type: String, required: true } },
    setup(props) {
      const { state, onConfirmEvent, onCancelEvent } = useConfirmHost()
      return () => [
        h(EntityList, { listId: props.listId }),
        h(ConfirmModal, {
          open: state.open,
          title: state.title,
          message: state.message,
          confirmLabel: state.confirmLabel,
          cancelLabel: state.cancelLabel,
          busy: state.busy,
          danger: state.danger,
          onConfirm: () => { onConfirmEvent().catch(() => {}) },
          onCancel: () => { onCancelEvent() },
        }),
      ]
    },
  })

  async function mountList(entities: Entity[]) {
    seedSchema()
    seedEntities(entities)
    const wrapper = mount(Host, {
      props: { listId },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    return wrapper
  }

  type Wrapper = Awaited<ReturnType<typeof mountList>>

  function rowCheckbox(wrapper: Wrapper, id: string) {
    return wrapper.find(`.rl-table-row[data-entity-id="${id}"] input[type="checkbox"]`)
  }

  async function select(wrapper: Wrapper, ...ids: string[]) {
    for (const id of ids) await rowCheckbox(wrapper, id).setValue(true)
    await flushPromises()
  }

  function bulkDelete(wrapper: Wrapper) {
    return wrapper.find('[data-testid="bulk-delete"]')
  }

  function press(key: string) {
    document.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }))
  }

  const toasts = () => useToasts().toasts.value

  it('has no per-row delete button', async () => {
    const wrapper = await mountList([makeEntity('T-1'), makeEntity('T-2')])
    expect(wrapper.find('.delete-btn').exists()).toBe(false)
    expect(wrapper.find('button[title="Delete"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('offers row selection when a row may be deleted, with no bulk actions configured', async () => {
    const wrapper = await mountList([makeEntity('T-1'), makeEntity('T-2')])
    expect(rowCheckbox(wrapper, 'T-1').exists()).toBe(true)
    wrapper.unmount()
  })

  it('offers no row selection when no row may be deleted and no action is configured', async () => {
    const wrapper = await mountList([
      makeEntity('T-1', { update: true, delete: false }),
      makeEntity('T-2', { update: true, delete: false }),
    ])
    expect(rowCheckbox(wrapper, 'T-1').exists()).toBe(false)
    wrapper.unmount()
  })

  it('deletes every selected row without a confirm, and toasts with Undo', async () => {
    const wrapper = await mountList([makeEntity('T-1'), makeEntity('T-2'), makeEntity('T-3')])
    await select(wrapper, 'T-1', 'T-3')

    await bulkDelete(wrapper).trigger('click')
    await flushPromises()

    expect(document.querySelector('[role="alertdialog"]')).toBeNull()
    expect(deleteEntityMock).toHaveBeenCalledTimes(2)
    expect(deleteEntityMock).toHaveBeenCalledWith(entityType, 'T-1')
    expect(deleteEntityMock).toHaveBeenCalledWith(entityType, 'T-3')
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]).toMatchObject({ title: 'Deleted 2 tickets', tone: 'success' })
    expect(toasts()[0].action?.label).toBe('Undo')
    // The selection is spent, so the bar leaves.
    await flushPromises()
    expect(bulkDelete(wrapper).exists()).toBe(false)
    wrapper.unmount()
  })

  it('leaves a selected row the principal may not delete out of the delete', async () => {
    const wrapper = await mountList([
      makeEntity('T-1'),
      makeEntity('T-2', { update: true, delete: false }),
    ])
    await select(wrapper, 'T-1', 'T-2')

    await bulkDelete(wrapper).trigger('click')
    await flushPromises()

    expect(deleteEntityMock).toHaveBeenCalledTimes(1)
    expect(deleteEntityMock).toHaveBeenCalledWith(entityType, 'T-1')
    wrapper.unmount()
  })

  it('hides Delete when no selected row may be deleted', async () => {
    const wrapper = await mountList([
      makeEntity('T-1'),
      makeEntity('T-2', { update: true, delete: false }),
    ])
    await select(wrapper, 'T-2')
    expect(bulkDelete(wrapper).exists()).toBe(false)
    wrapper.unmount()
  })

  it.each(['Delete', 'Backspace'])('%s deletes the selection', async (key) => {
    const wrapper = await mountList([makeEntity('T-1'), makeEntity('T-2')])
    await select(wrapper, 'T-2')
    // Ticking the box leaves focus on it; a checkbox is not a text field.
    ;(rowCheckbox(wrapper, 'T-2').element as HTMLInputElement).focus()

    press(key)
    await flushPromises()

    expect(deleteEntityMock).toHaveBeenCalledTimes(1)
    expect(deleteEntityMock).toHaveBeenCalledWith(entityType, 'T-2')
    wrapper.unmount()
  })

  it('does nothing on Delete when no row is selected', async () => {
    const wrapper = await mountList([makeEntity('T-1')])
    // Moving the cursor is not selecting.
    press('j')
    press('Delete')
    await flushPromises()
    expect(deleteEntityMock).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it.each(['Delete', 'Backspace'])('ignores %s while focus is in a text field', async (key) => {
    const wrapper = await mountList([makeEntity('T-1')])
    await select(wrapper, 'T-1')
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()

    press(key)
    await flushPromises()

    expect(deleteEntityMock).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('ignores Delete while focus is in an editable element', async () => {
    const wrapper = await mountList([makeEntity('T-1')])
    await select(wrapper, 'T-1')
    const editable = document.createElement('div')
    editable.contentEditable = 'true'
    // jsdom does not implement isContentEditable.
    Object.defineProperty(editable, 'isContentEditable', { value: true })
    editable.tabIndex = 0
    document.body.appendChild(editable)
    editable.focus()

    press('Delete')
    await flushPromises()

    expect(deleteEntityMock).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('keeps a row whose delete failed, keeps it selected, and names it', async () => {
    const wrapper = await mountList([makeEntity('T-1'), makeEntity('T-2')])
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    deleteEntityMock.mockImplementation((_type: string, id: string) =>
      id === 'T-2' ? Promise.reject(new Error('locked')) : Promise.resolve()
    )
    await select(wrapper, 'T-1', 'T-2')
    // The refetch after the delete: T-1 is gone, T-2 is still there.
    seedEntities([makeEntity('T-2')])

    await bulkDelete(wrapper).trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Title T-2')
    expect(wrapper.text()).not.toContain('Title T-1')
    expect((rowCheckbox(wrapper, 'T-2').element as HTMLInputElement).checked).toBe(true)
    const titles = toasts().map((t) => t.title)
    expect(titles).toContain('Deleted 1 ticket')
    expect(titles).toContain('Could not delete T-2: locked')
    consoleSpy.mockRestore()
    wrapper.unmount()
  })

  it('Undo restores each deleted row and refreshes the list', async () => {
    const wrapper = await mountList([makeEntity('T-1'), makeEntity('T-2')])
    await select(wrapper, 'T-1', 'T-2')
    await bulkDelete(wrapper).trigger('click')
    await flushPromises()
    const calls = listEntitiesMock.mock.calls.length

    toasts()[0].action!.onAction()
    await flushPromises()

    expect(restoreEntityMock).toHaveBeenCalledTimes(2)
    expect(restoreEntityMock).toHaveBeenCalledWith(entityType, 'T-1')
    expect(restoreEntityMock).toHaveBeenCalledWith(entityType, 'T-2')
    expect(listEntitiesMock.mock.calls.length).toBeGreaterThan(calls)
    expect(toasts().some((t) => t.title === 'Restored 2 tickets')).toBe(true)
    wrapper.unmount()
  })

  it('Undo against a server without restore shows an error toast', async () => {
    const wrapper = await mountList([makeEntity('T-1')])
    restoreEntityMock.mockRejectedValue(new ApiError('Not Found', { kind: 'http', status: 404, original: null }))
    await select(wrapper, 'T-1')
    await bulkDelete(wrapper).trigger('click')
    await flushPromises()

    toasts()[0].action!.onAction()
    await flushPromises()

    const error = toasts().find((t) => t.tone === 'danger')
    expect(error?.title).toBe('Could not restore T-1: it can no longer be restored')
    wrapper.unmount()
  })

  // A string `confirm:` is the dialog text; `true` keeps the default wording
  // (TKT-VVS16W).
  it.each([
    ['string', 'Close these tickets for good?', 'Close these tickets for good?'],
    ['true', true, 'Apply Close to 1 selected entities?'],
  ])('bulk action confirm: %s', async (_name, confirm, want) => {
    const schemaStore = useSchemaStore()
    const wrapper = await mountList([
      { ...makeEntity('T-1'), _actions: { update: true } },
    ])
    schemaStore.lists.set(listId, { ...schemaStore.getList(listId)!, actions: ['close'] } as never)
    schemaStore.actions.set('close', { label: 'Close', key: 'c', script: 'c.lua', confirm })
    await flushPromises()

    await select(wrapper, 'T-1')
    const btn = wrapper.findAll('[data-testid="bulk-action"]').find((b) => b.text().includes('Close'))
    expect(btn).toBeDefined()
    await btn!.trigger('click')
    await flushPromises()

    const dialog = document.querySelector<HTMLElement>('[role="alertdialog"]')
    expect(dialog).not.toBeNull()
    expect(dialog!.textContent).toContain(want)
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
    listEntitiesMock.mockReset().mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 25, has_more: false },
      included: {},
    })
    return listEntitiesMock
  }

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(entityType, 'tickets')
    deleteEntityMock.mockReset().mockResolvedValue(undefined)
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

    const wrapper = mount(withPageHeader(EntityList), { props: { listId }, attachTo: document.body, global: { plugins: [pinia, PiniaColada] } })
    await flushPromises()

    const input = wrapper.find<HTMLInputElement>('.search-box input[type="search"]')
    expect(input.element.value).toBe('foo')
    wrapper.unmount()
  })

  it('AC2: typing fires exactly one fetch after the debounce window', async () => {
    seedSchema()
    const fetchList = fakeFetchList()
    const wrapper = mount(withPageHeader(EntityList), { props: { listId }, attachTo: document.body, global: { plugins: [pinia, PiniaColada] } })
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

    const wrapper = mount(withPageHeader(EntityList), { props: { listId }, attachTo: document.body, global: { plugins: [pinia, PiniaColada] } })
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

// TKT-ODHV2D: incoming relation columns. Verifies getFormattedCellValue is
// direction-aware — an `direction: incoming` column reads the relation's
// inverse key from entity.relations and resolves the source IDs via the
// included map (exactly the same client-side ID→title path as outgoing
// columns). Rendered through the DOM since the function is component-local.
describe('EntityList incoming relation columns', () => {
  const listId = 'taaks'
  const entityType = 'taak'

  // A taak list with an incoming relation column (verantwoordelijk_voor points
  // persoon → taak; its inverse is verantwoordelijken).
  function seedSchema() {
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Taken',
      entity: entityType,
      columns: [
        { property: 'title', label: 'Title' },
        { relation: 'verantwoordelijk_voor', direction: 'incoming', label: 'Verantwoordelijken' },
      ],
    } as never)
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Taak',
      properties: { title: { type: 'string', values: null } },
    } as never)
    // The relation type declares an explicit inverse, so getInverseName
    // returns "verantwoordelijken" — the wire key the backend uses.
    schemaStore.relationTypes.set('verantwoordelijk_voor', {
      label: 'verantwoordelijk voor',
      from: ['persoon'],
      to: ['taak'],
      inverse: { id: 'verantwoordelijken' },
    } as never)
  }

  // Build a list response where the taak row carries incoming sources under
  // the inverse key, and the included map holds those source persoons.
  function seedIncomingResponse(sourceIds: string[]): ListResponse<Entity> {
    const included: Record<string, Entity> = {}
    for (const id of sourceIds) {
      included[id] = { id, type: 'persoon', properties: { title: `Name ${id}` }, _title: `Name ${id}` }
    }
    const response: ListResponse<Entity> = {
      data: [
        {
          id: 'TASK-1',
          type: entityType,
          properties: { title: 'Ship it' },
          relations: { verantwoordelijken: sourceIds },
        },
      ],
      meta: { total: 1, page: 1, per_page: 25, has_more: false },
      included,
    }
    listEntitiesMock.mockResolvedValue(response)
    return response
  }

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(entityType, 'taaks')
    listEntitiesMock.mockReset()
    deleteEntityMock.mockReset().mockResolvedValue(undefined)
    _resetModalStack()
    routerPush.mockClear()
    routerReplace.mockClear()
    mockRoute.query = {}
  })

  afterEach(() => {
    document.body.innerHTML = ''
    _resetModalStack()
  })

  it('resolves a single incoming source to its title', async () => {
    seedSchema()
    seedIncomingResponse(['PERS-JV'])
    const wrapper = mount(EntityList, {
      props: { listId },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    // The second column cell renders the resolved persoon title.
    //
    // `cellText` strips the stacked-row column label, which shares the cell
    // and would otherwise come back glued to the value.
    const dataCells = wrapper.findAll('.rl-table-row__cell')
    const relCell = dataCells[dataCells.length - 1]
    expect(cellText(relCell)).toBe('Name PERS-JV')
    wrapper.unmount()
  })

  it('resolves multiple incoming sources comma-separated', async () => {
    seedSchema()
    seedIncomingResponse(['PERS-JV', 'PERS-AB'])
    const wrapper = mount(EntityList, {
      props: { listId },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    const dataCells = wrapper.findAll('.rl-table-row__cell')
    const relCell = dataCells[dataCells.length - 1]
    expect(cellText(relCell)).toBe('Name PERS-JV, Name PERS-AB')
    wrapper.unmount()
  })

  it('outgoing lookup key stays empty for an incoming column (no collision)', async () => {
    seedSchema()
    // Row carries edges ONLY under the outgoing relation type, none under the
    // inverse. An incoming column must therefore render empty.
    listEntitiesMock.mockResolvedValue({
      data: [
        {
          id: 'TASK-1',
          type: entityType,
          properties: { title: 'Ship it' },
          relations: { verantwoordelijk_voor: ['PERS-JV'] },
        },
      ],
      meta: { total: 1, page: 1, per_page: 25, has_more: false },
      included: { 'PERS-JV': { id: 'PERS-JV', type: 'persoon', properties: {}, _title: 'Name' } },
    } as ListResponse<Entity>)
    const wrapper = mount(EntityList, {
      props: { listId },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    const dataCells = wrapper.findAll('.rl-table-row__cell')
    const relCell = dataCells[dataCells.length - 1]
    expect(cellText(relCell)).toBe('')
    wrapper.unmount()
  })
})

// Pins the anti-flash contract that TKT-TFSNBY's design depends on: a page
// change must NOT blank the table back to the block wait region.
//
// `loading` is computed from `isPending`, and a page change swaps to a NEW
// query key whose entry starts out `pending` — so the guard is Colada's
// `placeholderData: (prev) => prev`. When placeholder data is present Colada
// overrides the exposed state to `{status: 'success', data: placeholderData}`,
// which keeps `isPending` false and holds the previous page's rows on screen
// until the next page resolves.
//
// Without this test the behaviour is unpinned, and EntityList carried two
// contradictory comments about it (one claiming the spinner does show on a
// param change). Removing `placeholderData`, or gating the template on
// `asyncStatus`/`isLoading` instead of `isPending`, must fail here.
// The block wait region is RlStatusRegion's `pending` tone, whose spinner
// announces itself as a status. Keyed on that announcement rather than on a
// class name, so the contract survives the library restyling the region.
function blockWaitShown(wrapper: ReturnType<typeof mount>): boolean {
  return wrapper.find('[role="status"][aria-label="Loading"]').exists()
}

describe('EntityList pagination keeps previous rows (no spinner flash)', () => {
  const listId = 'tickets-list'
  const entityType = 'ticket'

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(entityType, 'tickets')
    listEntitiesMock.mockReset()
    mockRoute.query = {}
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Tickets',
      entity: entityType,
      page_size: 2,
      columns: [{ property: 'title', label: 'Title' }],
    } as never)
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Ticket',
      properties: { title: { type: 'string', values: null } },
    } as never)
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  function page(ids: string[], pageNo: number, hasMore: boolean): ListResponse<Entity> {
    return {
      data: ids.map((id) => ({ id, type: entityType, properties: { title: `Title ${id}` } })),
      meta: { total: 4, page: pageNo, per_page: 2, has_more: hasMore },
      included: {},
    }
  }

  it('holds page 1 rows on screen while page 2 is in flight', async () => {
    listEntitiesMock.mockResolvedValueOnce(page(['T-1', 'T-2'], 1, true))
    const wrapper = mount(EntityList, {
      props: { listId },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    expect(wrapper.findAll('.rl-table-row')).toHaveLength(2)
    expect(blockWaitShown(wrapper)).toBe(false)

    // Page 2 resolves only when we say so, so we can observe the in-flight gap.
    let resolvePage2: (r: ListResponse<Entity>) => void = () => {}
    listEntitiesMock.mockImplementationOnce(
      () => new Promise<ListResponse<Entity>>((res) => { resolvePage2 = res })
    )

    await wrapper.findComponent(Pagination).vm.$emit('page-change', 2)
    await flushPromises()

    // THE ASSERTION: mid-flight the old rows are still mounted and no
    // spinner replaced the table.
    expect(blockWaitShown(wrapper)).toBe(false)
    expect(wrapper.findAll('.rl-table-row')).toHaveLength(2)
    expect(wrapper.text()).toContain('Title T-1')

    resolvePage2(page(['T-3', 'T-4'], 2, false))
    await flushPromises()

    // Page 2's rows are now the real (non-placeholder) data.
    expect(blockWaitShown(wrapper)).toBe(false)
    expect(wrapper.text()).toContain('Title T-3')
    wrapper.unmount()
  })

  it('does show the spinner on the very first load (no previous data to hold)', async () => {
    let resolveFirst: (r: ListResponse<Entity>) => void = () => {}
    listEntitiesMock.mockImplementationOnce(
      () => new Promise<ListResponse<Entity>>((res) => { resolveFirst = res })
    )
    const wrapper = mount(EntityList, {
      props: { listId },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    // Cold load is the one case where the block spinner is correct.
    expect(blockWaitShown(wrapper)).toBe(true)

    resolveFirst(page(['T-1', 'T-2'], 1, true))
    await flushPromises()
    expect(blockWaitShown(wrapper)).toBe(false)
    wrapper.unmount()
  })
})

describe('EntityList compressed layout', () => {
  const listId = 'tickets'
  const entityType = 'ticket'

  /*
   * A list whose columns cover both sides of the `primary` decision: an enum
   * (status) and a face column, against a date (due) and a relation (blocks).
   */
  function seedSchema() {
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Tickets',
      entity: entityType,
      columns: [
        { property: 'title', label: 'Title' },
        { property: 'status', label: 'Status' },
        { face: true, label: 'Version' },
        { property: 'due', label: 'Due' },
        { relation: 'blocks', label: 'Blocks' },
      ],
    } as never)
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Ticket',
      faces: { draft: { label: 'Draft' } },
      properties: {
        title: { type: 'string', values: null },
        status: { type: 'enum', values: ['open', 'done'] },
        due: { type: 'date', values: null },
      },
    } as never)
    schemaStore.relationTypes.set('blocks', {
      label: 'blocks',
      from: ['ticket'],
      to: ['ticket'],
    } as never)
  }

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(entityType, 'tickets')
    listEntitiesMock.mockReset().mockResolvedValue({
      data: [
        {
          id: 'TKT-1',
          type: entityType,
          _title: 'First',
          properties: { title: 'First', status: 'open', due: '2026-01-01' },
        },
      ],
      meta: { total: 1, page: 1, per_page: 25, has_more: false },
      included: {},
    })
    deleteEntityMock.mockReset().mockResolvedValue(undefined)
    _resetModalStack()
    mockRoute.query = {}
  })

  afterEach(() => {
    document.body.innerHTML = ''
    _resetModalStack()
  })

  it('compresses rather than stacks, so a row stays one line beside the panel', async () => {
    seedSchema()
    const wrapper = mount(EntityList, {
      props: { listId },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    expect(wrapper.find('.rl-table-row--compress').exists()).toBe(true)
    expect(wrapper.find('.rl-table-row--stack').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps the enum badge and the face primary and drops the date and the relation', async () => {
    // `primary` is what survives the compressed layout. An enum or a face is
    // the state the list is scanned for; a date and a relation are in the
    // panel. Two secondary cells means the face column stayed primary.
    seedSchema()
    const wrapper = mount(EntityList, {
      props: { listId },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    const primary = wrapper
      .findAll('.rl-table-row__cell')
      .filter((c) => c.classes().some((k) => k.endsWith('--secondary')) === false)
    expect(primary.length).toBeGreaterThan(0)

    const secondary = wrapper.findAll('.rl-table-row__cell--secondary')
    expect(secondary.length).toBe(2)
    wrapper.unmount()
  })
})
