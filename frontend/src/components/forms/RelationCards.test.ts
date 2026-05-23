// TKT-G7N5 AC9: RelationCards consumes the per-relation-type
// affordance verdict (`:verdict` prop sourced from
// `entity._relations[type]` in DynamicForm). Verifies:
//
//   - `creatable === false` → + Add button absent
//   - `removable === false` → per-link x button absent on every link
//   - `fields[name].writable === false` → inline meta-field input
//     rendered with disabled attribute
//   - default (no verdict / verdict-fields-undefined) → all buttons
//     and inputs visible / enabled

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import RelationCards from './RelationCards.vue'
import { useSchemaStore } from '@/stores/schema'
import type { FormFieldOrRelation, RelationAffordance, Entity } from '@/types'

// Mock the API layer so we can return canned relations/entities
// without booting the network stack.
vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return {
    ...actual,
    getEntityRelations: vi.fn(),
    getEntity: vi.fn(),
  }
})

import { getEntityRelations, getEntity } from '@/api'

function seedSchema() {
  const schemaStore = useSchemaStore()
  schemaStore.entityTypes.set('ticket', {
    name: 'ticket',
    label: 'Ticket',
    properties: {},
  } as never)
  schemaStore.entityTypes.set('feature', {
    name: 'feature',
    label: 'Feature',
    properties: {},
  } as never)
  schemaStore.relationTypes.set('implements', {
    name: 'implements',
    from: ['ticket'],
    to: ['feature'],
    // Add a meta property so the inline-edit input renders.
    properties: { note: { type: 'string' } },
  } as never)
}

function seedRelations(targets: string[], metaByTarget: Record<string, Record<string, unknown>> = {}) {
  ;(getEntityRelations as ReturnType<typeof vi.fn>).mockResolvedValue(
    targets.map((id) => ({
      id,
      type: 'feature',
      meta: metaByTarget[id] || {},
    }))
  )
  ;(getEntity as ReturnType<typeof vi.fn>).mockImplementation((_, id: string) =>
    Promise.resolve({
      id,
      type: 'feature',
      properties: { title: id },
    } as Entity)
  )
}

async function mountCards(opts: {
  verdict?: RelationAffordance
  links?: string[]
}) {
  seedSchema()
  seedRelations(opts.links ?? ['FEAT-001'])
  const field: FormFieldOrRelation = {
    relation: 'implements',
    label: 'Implements',
    widget: 'cards',
    // Declare meta properties on the field so the inline-edit input
    // renders inside each card. Matches data-entry.yaml usage.
    properties: [{ property: 'note', label: 'Note' }],
  } as never
  const wrapper = mount(RelationCards, {
    props: {
      field,
      entityType: 'ticket',
      entityId: 'TKT-001',
      verdict: opts.verdict,
    },
    attachTo: document.body,
  })
  await flushPromises()
  return wrapper
}

describe('RelationCards affordance plumbing', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('default (no verdict): + Add button and per-link x button both visible', async () => {
    const wrapper = await mountCards({})
    expect(wrapper.find('.add-btn').exists()).toBe(true)
    expect(wrapper.find('.remove-btn').exists()).toBe(true)
    wrapper.unmount()
  })

  it('creatable=false: + Add button absent; remove still visible', async () => {
    const wrapper = await mountCards({
      verdict: { creatable: false },
    })
    expect(wrapper.find('.add-btn').exists()).toBe(false)
    expect(wrapper.find('.remove-btn').exists()).toBe(true)
    wrapper.unmount()
  })

  it('removable=false: per-link x button absent on every link; add still visible', async () => {
    const wrapper = await mountCards({
      verdict: { removable: false },
      links: ['FEAT-001', 'FEAT-002'],
    })
    // No remove button on ANY card.
    expect(wrapper.findAll('.remove-btn').length).toBe(0)
    expect(wrapper.find('.add-btn').exists()).toBe(true)
    wrapper.unmount()
  })

  it('both creatable=false and removable=false: both affordances hidden', async () => {
    const wrapper = await mountCards({
      verdict: { creatable: false, removable: false },
    })
    expect(wrapper.find('.add-btn').exists()).toBe(false)
    expect(wrapper.find('.remove-btn').exists()).toBe(false)
    wrapper.unmount()
  })

  it('fields.<meta>.writable=false: inline meta input rendered with disabled', async () => {
    const wrapper = await mountCards({
      verdict: {
        fields: { note: { writable: false } },
      },
      links: ['FEAT-001'],
    })
    const metaInput = wrapper.find('input.inline-edit')
    expect(metaInput.exists()).toBe(true)
    expect(metaInput.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('absent fields entry: meta input enabled (default writable)', async () => {
    const wrapper = await mountCards({
      verdict: { creatable: true }, // verdict present but no fields entry
    })
    const metaInput = wrapper.find('input.inline-edit')
    expect(metaInput.exists()).toBe(true)
    expect(metaInput.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
})
