import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

const listAllEntitiesMock = vi.fn()
const updateEntityMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
}))

const routerPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush }),
  useRoute: () => ({ query: {}, path: '/kanban/board' }),
  // Cards are real links since TKT-3CSZRG, so the component renders a
  // RouterLink; this mock replaces the whole module, so it must supply one.
  RouterLink: { template: '<a><slot /></a>' },
}))

vi.mock('@/composables/useBackTarget', () => ({
  useBackTarget: () => null,
}))

const KANBAN_ID = 'board'
const ENTITY_TYPE = 'ticket'

function makeTicket(id: string, title = `Ticket ${id}`): Entity {
  return {
    id,
    type: ENTITY_TYPE,
    properties: { title, status: 'todo' },
    relations: {},
  }
}

function seedSchema(configOverrides: Record<string, unknown> = {}) {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set(KANBAN_ID, {
    entity: ENTITY_TYPE,
    title: 'Board',
    column_property: 'status',
    columns: [{ value: 'todo', label: 'Todo' }],
    card: { title: 'title', fields: [] },
    ...configOverrides,
  } as never)
  schemaStore.entityTypes.set(ENTITY_TYPE, {
    name: ENTITY_TYPE,
    label: 'Ticket',
    properties: {
      title: { type: 'string', values: null },
      status: { type: 'enum', values: ['todo'] },
      team: { type: 'enum', values: ['alpha'] },
    },
  } as never)
}

function seedBoard(entities: Entity[]): ListResponse<Entity> {
  const response: ListResponse<Entity> = {
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
    included: {},
  }
  listAllEntitiesMock.mockResolvedValue(response)
  return response
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest(ENTITY_TYPE, 'tickets')
  listAllEntitiesMock.mockReset()
  updateEntityMock.mockReset().mockResolvedValue(undefined)
  routerPush.mockClear()
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountBoard(entities: Entity[], configOverrides: Record<string, unknown> = {}) {
  seedSchema(configOverrides)
  seedBoard(entities)
  const wrapper = mount(KanbanView, {
    props: { id: KANBAN_ID },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

// The card must be reachable and activatable by keyboard. It began as a bare
// <div> with @click only — unreachable and announced as nothing — was given
// role="button" + tabindex + a keydown handler, and is now a real RouterLink
// (TKT-3CSZRG), which provides focusability and Enter activation natively and
// additionally supports cmd/middle-click.
//
// These tests therefore assert the DESTINATION is right and the element is a
// genuine link, rather than re-testing the browser's own handling of <a>.
// Asserting a keydown handler now would pin a shim the element no longer needs.
describe('KanbanView card keyboard operability', () => {
  it('renders the card as a real link, so Enter activates it natively', async () => {
    const wrapper = await mountBoard([makeTicket('T-1')])
    const card = wrapper.find('.kanban-card')
    expect(card.exists()).toBe(true)

    // An <a> is focusable and Enter-activatable without any handler of ours.
    expect(card.element.tagName).toBe('A')
    expect(card.attributes('to') ?? card.attributes('href')).toBe('/entity/ticket/T-1')
  })

  it('points the card at the edit form when one is configured', async () => {
    const wrapper = await mountBoard([makeTicket('T-1')], { edit_form: 'ticket-form' })
    const card = wrapper.find('.kanban-card')

    expect(card.attributes('to') ?? card.attributes('href')).toBe('/form/ticket-form/T-1')
  })

  it('click and Enter reach the same destination', async () => {
    const wrapper = await mountBoard([makeTicket('T-1')])

    await wrapper.find('.kanban-card').trigger('click')
    const viaClick = routerPush.mock.calls[0]
    routerPush.mockClear()

    await wrapper.find('.kanban-card').trigger('keydown', { key: 'Enter' })
    const viaKeyboard = routerPush.mock.calls[0]

    expect(viaKeyboard).toEqual(viaClick)
  })

  it('ignores keys that are not Enter or Space', async () => {
    const wrapper = await mountBoard([makeTicket('T-1')])
    const card = wrapper.find('.kanban-card')

    await card.trigger('keydown', { key: 'a' })
    await card.trigger('keydown', { key: 'Tab' })
    await card.trigger('keydown', { key: 'ArrowDown' })

    expect(routerPush).not.toHaveBeenCalled()
  })

  // Space no longer needs intercepting: the card is an <a>, which does not
  // scroll the column on Space the way a focused div would, and Enter
  // activates it natively. Nothing of ours should cancel key events here.

  it('leaves a non-activating key event cancellable', async () => {
    const wrapper = await mountBoard([makeTicket('T-1')])
    const event = new KeyboardEvent('keydown', { key: 'Tab', cancelable: true, bubbles: true })

    wrapper.find('.kanban-card').element.dispatchEvent(event)
    await flushPromises()

    expect(event.defaultPrevented).toBe(false)
  })
})

describe('KanbanView accessible structure', () => {
  it('gives each card an accessible name and makes it focusable', async () => {
    const wrapper = await mountBoard([makeTicket('T-1', 'Fix the widget')])
    const card = wrapper.find('.kanban-card')

    // A real <a> is focusable and announced as a link without role/tabindex.
    // What is still ours to get right is the accessible NAME: without it the
    // card announces as its raw id, or as nothing.
    expect(card.element.tagName).toBe('A')
    expect(card.attributes('aria-label')).toBe('Fix the widget')
  })

  it('names the board region and labels each column by its heading', async () => {
    const wrapper = await mountBoard([makeTicket('T-1')])

    const board = wrapper.find('.kanban-board')
    expect(board.attributes('role')).toBe('group')
    expect(board.attributes('aria-label')).toBe('Board board')

    const column = wrapper.find('.kanban-column')
    expect(column.element.tagName).toBe('SECTION')
    const labelledBy = column.attributes('aria-labelledby')
    expect(labelledBy).toBe('kanban-col-todo')

    const heading = wrapper.find(`#${labelledBy}`)
    expect(heading.exists()).toBe(true)
    expect(heading.element.tagName).toBe('H2')
    expect(heading.text()).toContain('Todo')
  })

  it('renders the column cards as a list of list items', async () => {
    const wrapper = await mountBoard([makeTicket('T-1'), makeTicket('T-2')])

    const list = wrapper.find('.column-cards')
    expect(list.element.tagName).toBe('UL')
    // The <li> is the list item; the card link lives inside it, because a
    // RouterLink cannot itself render an <li>.
    expect(list.findAll('li.kanban-card-item')).toHaveLength(2)
    expect(list.findAll('li.kanban-card-item a.kanban-card')).toHaveLength(2)
  })

  it('labels each swimlane cell with its column and swimlane', async () => {
    const tickets = [makeTicket('T-1')]
    tickets[0].properties.team = 'alpha'
    const wrapper = await mountBoard(tickets, {
      swimlane_property: 'team',
      swimlanes: [{ value: 'alpha', label: 'Alpha' }],
    })

    const cell = wrapper.find('.swimlane-cell')
    expect(cell.exists()).toBe(true)
    expect(cell.element.tagName).toBe('UL')
    expect(cell.attributes('aria-label')).toBe('Todo — Alpha')
  })

  it('keeps swimlane cards keyboard-operable too', async () => {
    const tickets = [makeTicket('T-1')]
    tickets[0].properties.team = 'alpha'
    const wrapper = await mountBoard(tickets, {
      swimlane_property: 'team',
      swimlanes: [{ value: 'alpha', label: 'Alpha' }],
    })

    // Swimlane cards are links too — the same native focus/activation as the
    // column cards, pinned here because the swimlane branch is a separate
    // template block that has drifted from the column one before.
    const card = wrapper.find('.swimlane-cell .kanban-card')
    expect(card.element.tagName).toBe('A')
    expect(card.attributes('to') ?? card.attributes('href')).toBe('/entity/ticket/T-1')
  })

  it('keeps the card draggable so drag-and-drop is not regressed', async () => {
    const wrapper = await mountBoard([makeTicket('T-1')])

    expect(wrapper.find('.kanban-card').attributes('draggable')).toBe('true')
  })
})
