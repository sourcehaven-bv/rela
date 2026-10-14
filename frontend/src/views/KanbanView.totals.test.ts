import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { formatAmount } from '@/utils/format'
import type { Entity, KanbanConfig } from '@/types'

// TKT-SVLQQ6: enum column colours from `styles:`, `column_total:` under each
// column heading, and `currency:` on a card field.

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
}))

vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/kanban/pipeline' }),
}))

vi.mock('@/composables/useBackTarget', () => ({ useBackTarget: () => null }))

function deal(id: string, stage: string, value?: unknown): Entity {
  const properties: Record<string, unknown> = { title: id, stage }
  if (value !== undefined) properties.value = value
  return { id, type: 'deal', properties, relations: {} }
}

let pinia: ReturnType<typeof createPinia>

beforeEach(() => {
  localStorage.clear()
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest('deal', 'deals')
  listAllEntitiesMock.mockReset().mockResolvedValue({
    data: [
      deal('D-1', 'lead', 3500),
      deal('D-2', 'lead', 8000),
      deal('D-3', 'won', 1200),
      deal('D-4', 'won'), // no value: adds nothing
      deal('D-5', 'won', 'not a number'),
    ],
    meta: { total: 5, page: 1, per_page: 25, has_more: false },
  })
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountBoard(over: Partial<KanbanConfig> = {}) {
  const schema = useSchemaStore()
  schema.kanbans.set('pipeline', {
    entity: 'deal',
    title: 'Pipeline',
    column_property: 'stage',
    card: { title: 'title', fields: [] },
    ...over,
  } as never)
  schema.entityTypes.set('deal', {
    name: 'deal',
    properties: {
      title: { type: 'string' },
      stage: { type: 'deal_stage' },
      value: { type: 'integer' },
    },
  } as never)
  schema.customTypes.set('deal_stage', { values: ['lead', 'offer', 'won'] } as never)
  schema.styles = { deal_stage: { lead: 'badge-blue', won: 'badge-green' } }
  const wrapper = mount(KanbanView, {
    props: { id: 'pipeline' },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

function column(wrapper: Awaited<ReturnType<typeof mountBoard>>, id: string) {
  return wrapper.get(`.rl-board-column[data-section-id="${id}"]`)
}

describe('KanbanView enum column colour', () => {
  it('colours a column from its value style, and leaves an unstyled one alone', async () => {
    const wrapper = await mountBoard()
    expect(column(wrapper, 'lead').find('.rl-status-dot--blue').exists()).toBe(true)
    expect(column(wrapper, 'won').find('.rl-status-dot--green').exists()).toBe(true)
    // No style: the heading keeps the library's neutral dot.
    expect(column(wrapper, 'offer').find('.rl-status-dot--grey').exists()).toBe(true)
  })
})

describe('KanbanView column_total', () => {
  it('sums the property per column under the heading', async () => {
    const wrapper = await mountBoard({
      column_total: { property: 'value', label: 'Totaal', currency: 'EUR' },
    })
    const summary = (id: string) => column(wrapper, id).get('.rl-board-column__summary').text()
    expect(summary('lead')).toBe(`Totaal: ${formatAmount(11500, 'EUR')}`)
    // Missing and non-numeric values add nothing.
    expect(summary('won')).toBe(`Totaal: ${formatAmount(1200, 'EUR')}`)
    // An empty column still shows its total.
    expect(summary('offer')).toBe(`Totaal: ${formatAmount(0, 'EUR')}`)
  })

  it('defaults the label and shows a plain number without a currency', async () => {
    const wrapper = await mountBoard({ column_total: { property: 'value' } })
    expect(column(wrapper, 'lead').get('.rl-board-column__summary').text()).toBe(
      `Total: ${formatAmount(11500)}`,
    )
  })

  it('shows the total on a collapsed column', async () => {
    const wrapper = await mountBoard({
      columns: [{ value: 'lead', collapsed: true }, { value: 'won' }],
      column_total: { property: 'value', currency: 'EUR' },
    })
    const rail = wrapper.get('button[aria-label="Expand lead"]')
    expect(rail.get('.rl-board-column-collapsed__summary').text()).toBe(
      `Total: ${formatAmount(11500, 'EUR')}`,
    )
  })

  it('marks a total partial when a card hides the property from the reader', async () => {
    listAllEntitiesMock.mockResolvedValue({
      data: [
        deal('D-1', 'lead', 3500),
        { ...deal('D-2', 'lead'), _redacted: ['value'] },
        deal('D-3', 'won', 1200),
      ],
      meta: { total: 3, page: 1, per_page: 25, has_more: false },
    })
    const wrapper = await mountBoard({ column_total: { property: 'value', currency: 'EUR' } })
    const summary = (id: string) => column(wrapper, id).get('.rl-board-column__summary').text()
    expect(summary('lead')).toBe(`Total: ${formatAmount(3500, 'EUR')} (partial)`)
    expect(summary('won')).toBe(`Total: ${formatAmount(1200, 'EUR')}`)
  })

  it('shows no line without column_total', async () => {
    const wrapper = await mountBoard()
    expect(wrapper.find('.rl-board-column__summary').exists()).toBe(false)
  })
})

describe('KanbanView card currency', () => {
  it('renders an integer card field as money', async () => {
    const wrapper = await mountBoard({
      card: { title: 'title', fields: [{ property: 'value', currency: 'EUR', show_label: false }] },
    })
    const card = column(wrapper, 'lead').findAll('.kanban-card')[0]
    expect(card.get('.card-field').text()).toBe(formatAmount(3500, 'EUR'))
  })

  it('shows nothing for a card without a numeric value', async () => {
    const wrapper = await mountBoard({
      card: { title: 'title', fields: [{ property: 'value', currency: 'EUR' }] },
    })
    const cards = column(wrapper, 'won').findAll('.kanban-card')
    const noValue = cards.find((c) => c.text().includes('D-4'))!
    expect(noValue.find('.card-field').exists()).toBe(false)
  })
})
