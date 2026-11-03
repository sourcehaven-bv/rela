import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { formatAmount } from '@/utils/format'
import type { Entity, KanbanConfig } from '@/types'

// TKT-SVLQQ6: enum column colours from `styles:`, `column_summary:` under
// each column heading, and a `display: {currency}` property on a card.

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

async function mountBoard(over: Partial<KanbanConfig> = {}, currency: string | null = 'EUR') {
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
      value: { type: 'integer', ...(currency ? { display: { currency } } : {}) },
      closes: { type: 'date' },
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

describe('KanbanView column_summary', () => {
  const summary = (wrapper: Awaited<ReturnType<typeof mountBoard>>, id: string) =>
    column(wrapper, id)
      .findAll('.rl-board-column__summary p')
      .map((p) => p.text())

  it('renders one line per summary under the heading', async () => {
    const wrapper = await mountBoard({
      column_summary: [
        { function: 'count', label: 'Deals' },
        { function: 'sum', property: 'value', label: 'Totaal' },
      ],
    })
    expect(summary(wrapper, 'lead')).toEqual(['Deals: 2', `Totaal: ${formatAmount(11500, 'EUR')}`])
    // Missing and non-numeric values add nothing, but the cards still count.
    expect(summary(wrapper, 'won')).toEqual(['Deals: 3', `Totaal: ${formatAmount(1200, 'EUR')}`])
  })

  it('shows 0 for count and sum, and a dash for avg, min and max, on an empty column', async () => {
    const wrapper = await mountBoard({
      column_summary: [
        { function: 'count' },
        { function: 'sum', property: 'value' },
        { function: 'avg', property: 'value' },
        { function: 'min', property: 'value' },
        { function: 'max', property: 'value' },
      ],
    })
    expect(summary(wrapper, 'offer')).toEqual([
      'Count: 0',
      `Total: ${formatAmount(0, 'EUR')}`,
      'Average: –',
      'Minimum: –',
      'Maximum: –',
    ])
  })

  it('formats avg, min and max of a currency property as money', async () => {
    const wrapper = await mountBoard({
      column_summary: [
        { function: 'avg', property: 'value' },
        { function: 'min', property: 'value' },
        { function: 'max', property: 'value' },
      ],
    })
    expect(summary(wrapper, 'lead')).toEqual([
      `Average: ${formatAmount(5750, 'EUR')}`,
      `Minimum: ${formatAmount(3500, 'EUR')}`,
      `Maximum: ${formatAmount(8000, 'EUR')}`,
    ])
  })

  it('shows a plain number for a property without a currency', async () => {
    const wrapper = await mountBoard({ column_summary: [{ function: 'sum', property: 'value' }] }, null)
    expect(summary(wrapper, 'lead')).toEqual([`Total: ${formatAmount(11500)}`])
  })

  it('shows the summaries compactly on a collapsed column', async () => {
    const wrapper = await mountBoard({
      columns: [{ value: 'lead', collapsed: true }, { value: 'won' }],
      column_summary: [{ function: 'count' }, { function: 'sum', property: 'value' }],
    })
    const rail = wrapper.get('button[aria-label="Expand lead"]')
    expect(rail.get('.rl-board-column-collapsed__summary').text()).toBe(
      `Count: 2 · Total: ${formatAmount(11500, 'EUR')}`,
    )
  })

  it('marks a summary partial when a card hides the property from the reader', async () => {
    listAllEntitiesMock.mockResolvedValue({
      data: [
        deal('D-1', 'lead', 3500),
        { ...deal('D-2', 'lead'), _redacted: ['value'] },
        deal('D-3', 'won', 1200),
      ],
      meta: { total: 3, page: 1, per_page: 25, has_more: false },
    })
    const wrapper = await mountBoard({
      column_summary: [{ function: 'count' }, { function: 'sum', property: 'value' }],
    })
    // The count covers every card the reader sees, so it is never partial.
    expect(summary(wrapper, 'lead')).toEqual(['Count: 2', `Total: ${formatAmount(3500, 'EUR')} (partial)`])
    expect(summary(wrapper, 'won')).toEqual(['Count: 1', `Total: ${formatAmount(1200, 'EUR')}`])
  })

  it('shows no summary without column_summary', async () => {
    const wrapper = await mountBoard()
    expect(wrapper.find('.rl-board-column__summary').exists()).toBe(false)
  })
})

describe('KanbanView currency card field', () => {
  it('renders a currency property on a card as money', async () => {
    const wrapper = await mountBoard({
      card: { title: 'title', fields: [{ property: 'value', show_label: false }] },
    })
    const card = column(wrapper, 'lead').findAll('.kanban-card')[0]
    expect(card.get('.card-field').text()).toBe(formatAmount(3500, 'EUR'))
  })

  it('shows nothing for a card without a value', async () => {
    const wrapper = await mountBoard({
      card: { title: 'title', fields: [{ property: 'value' }] },
    })
    const cards = column(wrapper, 'won').findAll('.kanban-card')
    const noValue = cards.find((c) => c.text().includes('D-4'))!
    expect(noValue.find('.card-field').exists()).toBe(false)
  })
})
