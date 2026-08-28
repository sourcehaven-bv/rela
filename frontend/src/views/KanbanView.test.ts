import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// KanbanView fetches its board through the api layer (useQuery over
// listAllEntities) and moves cards via updateEntity. Mock the api functions,
// mirroring EntityList.test.ts.
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
}))

vi.mock('@/composables/useBackTarget', () => ({
  useBackTarget: () => null,
}))

const KANBAN_ID = 'board'
const ENTITY_TYPE = 'ticket'

function makeTicket(id: string): Entity {
  return {
    id,
    type: ENTITY_TYPE,
    properties: { title: `Ticket ${id}`, status: 'todo' },
    relations: {},
  }
}

// seedSchema installs a kanban config whose card renders the given fields,
// plus a `verantwoordelijk_voor` relation type with a declared inverse so the
// incoming-direction path resolves via getInverseName.
function seedSchema(
  fields: Array<Record<string, unknown>>,
  configOverrides: Record<string, unknown> = {}
) {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set(KANBAN_ID, {
    entity: ENTITY_TYPE,
    title: 'Board',
    column_property: 'status',
    columns: [{ value: 'todo', label: 'Todo' }],
    card: { title: 'title', fields },
    ...configOverrides,
  } as never)
  schemaStore.entityTypes.set(ENTITY_TYPE, {
    name: ENTITY_TYPE,
    label: 'Ticket',
    properties: {
      title: { type: 'string', values: null },
      status: { type: 'enum', values: ['todo'] },
      // Enum property most tickets leave unset — used by the
      // unset-field-suppression test below.
      effort: { type: 'enum', values: ['s', 'm', 'l'] },
    },
  } as never)
  // Relation with a declared inverse: incoming edges land under `handled_by`.
  schemaStore.relationTypes.set('verantwoordelijk_voor', {
    id: 'verantwoordelijk_voor',
    inverse: { id: 'handled_by' },
  } as never)
}

function seedBoard(entities: Entity[], included: Record<string, Entity> = {}): ListResponse<Entity> {
  const response: ListResponse<Entity> = {
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
    included,
  }
  listAllEntitiesMock.mockResolvedValue(response)
  return response
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest(ENTITY_TYPE, 'tickets')
  _setEntityPluralForTest('person', 'people')
  listAllEntitiesMock.mockReset()
  updateEntityMock.mockReset().mockResolvedValue(undefined)
  routerPush.mockClear()
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountBoard(
  fields: Array<Record<string, unknown>>,
  entities: Entity[],
  included: Record<string, Entity> = {},
  configOverrides: Record<string, unknown> = {}
) {
  seedSchema(fields, configOverrides)
  seedBoard(entities, included)
  const wrapper = mount(KanbanView, {
    props: { id: KANBAN_ID },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

// NOTE (RR-UKS8BW): these are SPA-only unit tests. They prove that GIVEN the
// wire shape (a `relations` map with the expected key + an `included` map) the
// component resolves and renders the target — nothing more. The listAllEntities
// mock injects those maps directly; it is NOT the real endpoint.
//
// In particular the two INCOMING cases inject the inverse key
// (`relations: { handled_by: [...] }` / `{ blocks_inverse: [...] }`) that the
// real list endpoint does NOT emit on this branch — that key is populated
// server-side only once TKT-ODHV2D merges. So a green here is NOT end-to-end
// proof that incoming card fields work; it assumes the ODHV2D wire contract.
// The server side of that contract is pinned by the Go test
// `TestListEndpoint_IncomingEdge_InverseKey_ODHV2DContract` in
// internal/dataentry, which skips until ODHV2D lands.
describe('KanbanView card relation fields (assumes TKT-ODHV2D wire contract)', () => {
  it('renders an outgoing relation target title resolved from included', async () => {
    const ticket = makeTicket('T-1')
    // Outgoing edge keyed by the relation name itself.
    ticket.relations = { verantwoordelijk_voor: ['PER-9'] }
    const included = {
      'PER-9': { id: 'PER-9', type: 'person', _title: 'Alice', properties: {} } as Entity,
    }
    const wrapper = await mountBoard(
      [{ relation: 'verantwoordelijk_voor', direction: 'outgoing', label: 'Owner' }],
      [ticket],
      included
    )

    const card = wrapper.find('.kanban-card')
    expect(card.exists()).toBe(true)
    expect(card.text()).toContain('Owner')
    expect(card.text()).toContain('Alice')
    // Requested includes because a card field is a relation.
    expect(listAllEntitiesMock).toHaveBeenCalledWith(
      ENTITY_TYPE,
      { include: '*' },
      expect.any(AbortSignal)
    )
    wrapper.unmount()
  })

  it('renders an incoming relation target via the declared inverse key (ODHV2D contract)', async () => {
    const ticket = makeTicket('T-2')
    // ODHV2D CONTRACT: incoming edges are serialized under the relation's
    // inverse (handled_by). The real list endpoint only emits this key once
    // TKT-ODHV2D merges (see the Go contract test); here we inject it to test
    // the SPA's resolution given that shape.
    ticket.relations = { handled_by: ['PER-7'] }
    const included = {
      'PER-7': { id: 'PER-7', type: 'person', _title: 'Bob', properties: {} } as Entity,
    }
    const wrapper = await mountBoard(
      [{ relation: 'verantwoordelijk_voor', direction: 'incoming' }],
      [ticket],
      included
    )

    const card = wrapper.find('.kanban-card')
    expect(card.text()).toContain('Bob')
    wrapper.unmount()
  })

  it('falls back to <relation>_inverse when no inverse is declared (ODHV2D contract)', async () => {
    const schemaStore = useSchemaStore()
    const ticket = makeTicket('T-3')
    // ODHV2D CONTRACT: same as above — the `<relation>_inverse` key is a
    // server-side emission that only exists once TKT-ODHV2D merges; injected
    // here to exercise the SPA fallback key derivation.
    ticket.relations = { blocks_inverse: ['PER-5'] }
    const included = {
      'PER-5': { id: 'PER-5', type: 'person', _title: 'Carol', properties: {} } as Entity,
    }
    seedSchema([{ relation: 'blocks', direction: 'incoming' }])
    // `blocks` has no declared inverse.
    schemaStore.relationTypes.set('blocks', { id: 'blocks' } as never)
    seedBoard([ticket], included)
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    expect(wrapper.find('.kanban-card').text()).toContain('Carol')
    wrapper.unmount()
  })

  it('falls back to the raw id when a relation target is not in included', async () => {
    const ticket = makeTicket('T-4')
    ticket.relations = { verantwoordelijk_voor: ['PER-404'] }
    const wrapper = await mountBoard(
      [{ relation: 'verantwoordelijk_voor', direction: 'outgoing' }],
      [ticket],
      {}
    )
    expect(wrapper.find('.kanban-card').text()).toContain('PER-404')
    wrapper.unmount()
  })
})

describe('KanbanView card property fields', () => {
  it('renders a plain property value and does not request includes', async () => {
    const ticket = makeTicket('T-5')
    ticket.properties = { title: 'Ticket T-5', status: 'todo', priority: 'high' }
    const wrapper = await mountBoard([{ property: 'priority', label: 'Priority' }], [ticket])

    const card = wrapper.find('.kanban-card')
    expect(card.text()).toContain('Priority')
    expect(card.text()).toContain('high')
    // No relation field → no include param (property-only boards unchanged).
    expect(listAllEntitiesMock).toHaveBeenCalledWith(
      ENTITY_TYPE,
      undefined,
      expect.any(AbortSignal)
    )
    wrapper.unmount()
  })

  it('renders an enum property as a Badge', async () => {
    const ticket = makeTicket('T-6')
    ticket.properties = { title: 'Ticket T-6', status: 'todo' }
    const wrapper = await mountBoard([{ property: 'status' }], [ticket])

    // Badge component renders the enum value inside the card.
    const card = wrapper.find('.kanban-card')
    expect(card.find('.badge').exists()).toBe(true)
    expect(card.text()).toContain('todo')
    wrapper.unmount()
  })
})

describe('KanbanView column header enum labels', () => {
  // Seed a board grouped by `status` whose column carries no explicit label
  // (label === value, the auto-generated shape) and whose status enum defines
  // a display label. The header must resolve the enum label.
  function seedLabeledBoard() {
    const schemaStore = useSchemaStore()
    schemaStore.kanbans.set(KANBAN_ID, {
      entity: ENTITY_TYPE,
      title: 'Board',
      column_property: 'status',
      columns: [{ value: 'todo', label: 'todo' }],
      card: { title: 'title', fields: [{ property: 'status' }] },
    } as never)
    schemaStore.entityTypes.set(ENTITY_TYPE, {
      name: ENTITY_TYPE,
      label: 'Ticket',
      properties: {
        title: { type: 'string' },
        status: { type: 'enum', values: ['todo'], labels: { todo: 'To Do' } },
      },
    } as never)
  }

  it('shows the enum label in the column header when no explicit column label', async () => {
    seedLabeledBoard()
    const ticket = makeTicket('T-7')
    ticket.properties = { title: 'Ticket T-7', status: 'todo' }
    seedBoard([ticket])
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    expect(wrapper.find('.column-title').text()).toBe('To Do')
    wrapper.unmount()
  })

  it('keeps an explicit kanban column label over the enum label', async () => {
    const schemaStore = useSchemaStore()
    seedLabeledBoard()
    // Override the column with an explicit, different label.
    schemaStore.kanbans.set(KANBAN_ID, {
      entity: ENTITY_TYPE,
      title: 'Board',
      column_property: 'status',
      columns: [{ value: 'todo', label: 'Backlog' }],
      card: { title: 'title', fields: [{ property: 'status' }] },
    } as never)
    const ticket = makeTicket('T-8')
    ticket.properties = { title: 'Ticket T-8', status: 'todo' }
    seedBoard([ticket])
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    expect(wrapper.find('.column-title').text()).toBe('Backlog')
    wrapper.unmount()
  })
})

// Card fields whose value is unset must not render at all: an enum field
// with an empty value produced a dangling "effort:" label plus an empty
// gray Badge pill on every card that hadn't set the property.
describe('KanbanView card fields with unset values', () => {
  it('drops rows for unset fields instead of rendering an empty badge', async () => {
    const ticket = makeTicket('T-9') // status set, effort absent
    const wrapper = await mountBoard([{ property: 'status' }, { property: 'effort' }], [ticket])

    const card = wrapper.find('.kanban-card')
    expect(card.exists()).toBe(true)
    const labels = card.findAll('.field-label').map((n) => n.text())
    expect(labels).toContain('status:')
    expect(labels).not.toContain('effort:')
    wrapper.unmount()
  })

  it('renders the row normally once the field has a value', async () => {
    const ticket = makeTicket('T-10')
    ticket.properties.effort = 'm'
    const wrapper = await mountBoard([{ property: 'effort' }], [ticket])

    const card = wrapper.find('.kanban-card')
    const labels = card.findAll('.field-label').map((n) => n.text())
    expect(labels).toContain('effort:')
    expect(card.text()).toContain('m')
    wrapper.unmount()
  })
})

// Admin-authored header/footer info regions (TKT-6S331G). These mirror the
// list-view regions and share both the resolvers (viewHeaderMarkdown /
// viewFooterMarkdown) and the .view-info styles with EntityList.
describe('KanbanView info regions (header/footer)', () => {
  it('renders header markdown above the board', async () => {
    const wrapper = await mountBoard([], [makeTicket('T-1')], {}, {
      header: 'Cards move **left to right**.',
    })

    const header = wrapper.find('.view-info--top')
    expect(header.exists()).toBe(true)
    expect(header.html()).toContain('<strong>left to right</strong>')
    wrapper.unmount()
  })

  it('renders footer markdown below the board', async () => {
    const wrapper = await mountBoard([], [makeTicket('T-1')], {}, {
      footer: 'See the [runbook](https://example.test/runbook).',
    })

    const footer = wrapper.find('.view-info--bottom')
    expect(footer.exists()).toBe(true)
    expect(footer.html()).toContain('href="https://example.test/runbook"')
    wrapper.unmount()
  })

  it('routes admin markdown through the sanitizer before v-html', async () => {
    const wrapper = await mountBoard([], [makeTicket('T-1')], {}, {
      // Two separate payloads on purpose — see the note below. The header
      // proves markdown is processed; the footer proves scripts are stripped.
      header: '**bold**',
      footer: '<script>alert(3)</script>',
    })

    // What this proves: renderMarkdown (marked + DOMPurify) is actually IN the
    // path between data-entry.yaml and v-html — the markdown was processed
    // (<strong> exists, so the config string was not raw-passed to v-html) and
    // a script element did not survive.
    //
    // What this deliberately does NOT assert: inline event handlers and
    // javascript: hrefs. Under this suite's happy-dom environment DOMPurify
    // leaves BOTH intact when the input parses to multiple nodes
    // (`<p>x</p>\n<img onerror=...>` keeps onerror; `<a href="javascript:...">`
    // keeps the href) — and for the same reason it also keeps a script when one
    // follows inline content (`**bold** <script>` survives, a bare `<script>`
    // does not, which is why the two payloads above are kept apart). That is a
    // happy-dom DOM defect, not a renderMarkdown bug: every one of those
    // payloads through DOMPurify under jsdom, and in a real browser, comes back
    // sanitized. Asserting them here would fail for an environment reason while
    // proving nothing about production, so the real coverage for handler/href
    // stripping is DOMPurify's own suite plus manual browser verification.
    const html = wrapper.html()
    expect(html).toContain('<strong>bold</strong>')
    expect(html).not.toContain('<script>')
    expect(wrapper.find('.view-info--top script').exists()).toBe(false)
    expect(wrapper.find('.view-info--bottom script').exists()).toBe(false)
    wrapper.unmount()
  })

  it('renders neither region when header and footer are unset', async () => {
    const wrapper = await mountBoard([], [makeTicket('T-1')])

    expect(wrapper.find('.view-info--top').exists()).toBe(false)
    expect(wrapper.find('.view-info--bottom').exists()).toBe(false)
    wrapper.unmount()
  })

  it('treats whitespace-only header/footer as unset', async () => {
    const wrapper = await mountBoard([], [makeTicket('T-1')], {}, {
      header: '   \n',
      footer: '\t',
    })

    expect(wrapper.find('.view-info--top').exists()).toBe(false)
    expect(wrapper.find('.view-info--bottom').exists()).toBe(false)
    wrapper.unmount()
  })

  it('renders the footer outside the scrolling board container', async () => {
    // AC6: the footer must not live inside .kanban-board, which owns the
    // horizontal scroll — otherwise it scrolls off-screen on a wide board.
    // jsdom does not lay out, so this asserts containment structurally.
    const wrapper = await mountBoard([], [makeTicket('T-1')], {}, {
      footer: 'stays put',
    })

    expect(wrapper.find('.kanban-board .view-info--bottom').exists()).toBe(false)
    expect(wrapper.find('.view-info--bottom').exists()).toBe(true)
    wrapper.unmount()
  })

  it('renders the footer even when the board fails to load', async () => {
    seedSchema([], { footer: 'still here' })
    listAllEntitiesMock.mockRejectedValue(new Error('boom'))
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    expect(wrapper.find('.error-state').exists()).toBe(true)
    expect(wrapper.find('.view-info--bottom').exists()).toBe(true)
    wrapper.unmount()
  })
})
