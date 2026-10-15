import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { formatAmount, formatDate } from '@/utils/format'
import type { Entity, KanbanCardField } from '@/types'

// TKT-8Z2LT3: a card field `{relation, direction, property}` shows that
// property of each related entity, rendered with the related type's widget,
// enum styles and display format.

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

function deal(id: string, relations: Record<string, string[]> = {}): Entity {
  return { id, type: 'deal', properties: { title: id, stage: 'lead' }, relations }
}

function org(id: string, properties: Record<string, unknown>): Entity {
  return { id, type: 'org', properties: { name: id, ...properties } }
}

const SEGMENT: KanbanCardField = {
  relation: 'has_deal',
  direction: 'incoming',
  property: 'segment',
  show_label: false,
}

let pinia: ReturnType<typeof createPinia>

beforeEach(() => {
  localStorage.clear()
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest('deal', 'deals')
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountBoard(
  data: Entity[],
  included: Record<string, Entity>,
  fields: KanbanCardField[]
) {
  listAllEntitiesMock.mockReset().mockResolvedValue({
    data,
    included,
    meta: { total: data.length, page: 1, per_page: 25, has_more: false },
  })
  const schema = useSchemaStore()
  schema.kanbans.set('pipeline', {
    entity: 'deal',
    title: 'Pipeline',
    column_property: 'stage',
    columns: [{ value: 'lead' }],
    card: { title: 'title', fields },
  } as never)
  schema.entityTypes.set('deal', {
    name: 'deal',
    properties: { title: { type: 'string' }, stage: { type: 'enum', values: ['lead'] } },
  } as never)
  schema.entityTypes.set('org', {
    name: 'org',
    properties: {
      name: { type: 'string' },
      // The schema API inlines a custom type's values on the property.
      segment: { type: 'segment', values: ['hospital', 'government'] },
      revenue: { type: 'integer', display: { currency: 'EUR' } },
      since: { type: 'date' },
      city: { type: 'string' },
      tags: { type: 'segment', values: ['hospital', 'government'], list: true },
    },
  } as never)
  schema.entityTypes.set('lead', {
    name: 'lead',
    properties: { city: { type: 'string' } },
  } as never)
  schema.customTypes.set('segment', { values: ['hospital', 'government'] } as never)
  schema.styles = { segment: { hospital: 'badge-blue', government: 'badge-green' } }
  schema.relationTypes.set('has_deal', {
    label: 'has deal',
    from: ['org', 'lead'],
    to: ['deal'],
    inverse: { id: 'deal_of' },
  } as never)
  schema.relationTypes.set('handled_by', {
    label: 'handled by',
    from: ['deal'],
    to: ['org'],
  } as never)
  const wrapper = mount(KanbanView, {
    props: { id: 'pipeline' },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

function card(wrapper: Awaited<ReturnType<typeof mountBoard>>, title: string) {
  return wrapper.findAll('.kanban-card').find((c) => c.text().includes(title))!
}

describe('KanbanView related-property card field', () => {
  it('asks the server to include related entities', async () => {
    await mountBoard([deal('D-1')], {}, [SEGMENT])
    expect(listAllEntitiesMock.mock.calls[0][1]).toMatchObject({ include: '*' })
  })

  it('shows an incoming related enum as a badge in its styled colour', async () => {
    const wrapper = await mountBoard(
      [deal('D-1', { deal_of: ['O-1'] })],
      { 'O-1': org('O-1', { segment: 'hospital' }) },
      [SEGMENT]
    )
    const badge = card(wrapper, 'D-1').get('.card-field .badge')
    expect(badge.text()).toBe('hospital')
    expect(badge.classes()).toContain('badge--blue')
  })

  it('shows each distinct value once, in relation order', async () => {
    const wrapper = await mountBoard(
      [deal('D-1', { deal_of: ['O-1', 'O-2', 'O-3'] })],
      {
        'O-1': org('O-1', { segment: 'hospital' }),
        'O-2': org('O-2', { segment: 'government' }),
        'O-3': org('O-3', { segment: 'hospital' }),
      },
      [SEGMENT]
    )
    const badges = card(wrapper, 'D-1').findAll('.card-field .badge')
    expect(badges.map((b) => b.text())).toEqual(['hospital', 'government'])
    expect(badges[1].classes()).toContain('badge--green')
  })

  it('shows nothing when no related entity carries the value', async () => {
    // O-2 stands for a related entity whose segment the reader may not see:
    // the server strips it from `included`, so the property is absent. O-9 is
    // not in `included` at all, as for a related entity the reader cannot see.
    const wrapper = await mountBoard(
      [deal('D-1', { deal_of: ['O-2', 'O-9'] }), deal('D-2')],
      { 'O-2': org('O-2', {}) },
      [SEGMENT]
    )
    expect(card(wrapper, 'D-1').find('.card-field').exists()).toBe(false)
    expect(card(wrapper, 'D-1').text()).not.toContain('O-9')
    expect(card(wrapper, 'D-2').find('.card-field').exists()).toBe(false)
  })

  it('formats an outgoing related currency and date with the related type', async () => {
    const wrapper = await mountBoard(
      [deal('D-1', { handled_by: ['O-1'] })],
      { 'O-1': org('O-1', { revenue: 3500, since: '2026-03-01' }) },
      [
        { relation: 'handled_by', direction: 'outgoing', property: 'revenue', label: 'Omzet' },
        { relation: 'handled_by', direction: 'outgoing', property: 'since' },
      ]
    )
    const lines = card(wrapper, 'D-1').findAll('.card-field')
    expect(lines[0].text()).toBe(`Omzet: ${formatAmount(3500, 'EUR')}`)
    // Labelled by the property when no label is set.
    expect(lines[1].text()).toBe(`since: ${formatDate('2026-03-01')}`)
  })

  it('shows each element of a list property once across related entities', async () => {
    const wrapper = await mountBoard(
      [deal('D-1', { deal_of: ['O-1', 'O-2'] })],
      {
        'O-1': org('O-1', { tags: ['hospital', 'government'] }),
        'O-2': org('O-2', { tags: ['government'] }),
      },
      [{ ...SEGMENT, property: 'tags' }]
    )
    const badges = card(wrapper, 'D-1').findAll('.card-field .badge')
    expect(badges.map((b) => b.text())).toEqual(['hospital', 'government'])
    expect(badges[0].classes()).toContain('badge--blue')
  })

  it('joins plain text values with commas, and counts a value once across types', async () => {
    const wrapper = await mountBoard(
      [deal('D-1', { deal_of: ['O-1', 'L-1', 'O-2'] })],
      {
        'O-1': org('O-1', { city: 'Utrecht' }),
        'L-1': { id: 'L-1', type: 'lead', properties: { city: 'Utrecht' } },
        'O-2': org('O-2', { city: 'Breda' }),
      },
      [{ ...SEGMENT, property: 'city' }]
    )
    expect(card(wrapper, 'D-1').get('.card-field').text()).toBe('Utrecht, Breda')
  })

  it('keeps a title field and a property field on the same relation apart', async () => {
    const wrapper = await mountBoard(
      [deal('D-1', { deal_of: ['O-1'] })],
      { 'O-1': { ...org('O-1', { segment: 'hospital' }), _title: 'Ziekenhuis A' } },
      [{ relation: 'has_deal', direction: 'incoming', show_label: false }, SEGMENT]
    )
    const lines = card(wrapper, 'D-1').findAll('.card-field')
    expect(lines.map((l) => l.text())).toEqual(['Ziekenhuis A', 'hospital'])
  })
})
