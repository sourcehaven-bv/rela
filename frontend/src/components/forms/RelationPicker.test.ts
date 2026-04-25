import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import RelationPicker from './RelationPicker.vue'
import { useSchemaStore } from '@/stores/schema'
import { useEntitiesStore } from '@/stores/entities'
import type { Entity } from '@/types'
import type { FormFieldOrRelation } from '@/types/config'

function seedSchema(targetType = 'ticket') {
  const schemaStore = useSchemaStore()
  schemaStore.entityTypes.set(targetType, {
    name: targetType,
    label: 'Ticket',
    properties: {},
  } as never)
  schemaStore.relationTypes.set('affects', {
    name: 'affects',
    from: ['ticket'],
    to: [targetType],
    max_outgoing: 1,
  } as never)
}

function seedCandidates(entities: Entity[]) {
  const entitiesStore = useEntitiesStore()
  entitiesStore.fetchList = vi.fn().mockResolvedValue({
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 100, has_more: false },
    included: {},
  })
}

function entity(id: string, title?: unknown): Entity {
  return {
    id,
    type: 'ticket',
    properties: title === undefined ? {} : { title: title as string },
  }
}

async function mountPicker(value: string[], candidates: Entity[]) {
  seedSchema()
  seedCandidates(candidates)
  const field: FormFieldOrRelation = { relation: 'affects', label: 'Affects' }
  const wrapper = mount(RelationPicker, {
    props: { field, entityType: 'ticket', value },
    attachTo: document.body,
  })
  await flushPromises()
  return wrapper
}

describe('RelationPicker — entity label rendering', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('selected chip shows "Title (ID)" when entity has a title', async () => {
    const e = entity('TKT-001', 'Fix login bug')
    const wrapper = await mountPicker([e.id], [e])

    const chip = wrapper.find('.selected-entity .entity-label')
    expect(chip.exists()).toBe(true)
    expect(chip.text()).toBe('Fix login bug (TKT-001)')
    wrapper.unmount()
  })

  it('selected chip shows id alone when title is missing', async () => {
    const e = entity('TKT-002')
    const wrapper = await mountPicker([e.id], [e])

    const chip = wrapper.find('.selected-entity .entity-label')
    expect(chip.text()).toBe('TKT-002')
    expect(chip.text()).not.toContain('(')
    wrapper.unmount()
  })

  it('selected chip shows id alone when title is empty / whitespace', async () => {
    const blank = entity('TKT-003', '')
    const ws = entity('TKT-004', '   ')
    const wrapper = await mountPicker([blank.id, ws.id], [blank, ws])

    const chips = wrapper.findAll('.selected-entity .entity-label')
    expect(chips.map((c) => c.text())).toEqual(['TKT-003', 'TKT-004'])
    wrapper.unmount()
  })

  it('selected chip shows id alone when title is non-string (does not stringify object)', async () => {
    // Entity.properties is Record<string, unknown>; guard against object-typed
    // values that would otherwise render as "[object Object]".
    const e: Entity = {
      id: 'TKT-005',
      type: 'ticket',
      properties: { title: { unexpected: 'shape' } },
    }
    const wrapper = await mountPicker([e.id], [e])

    const chip = wrapper.find('.selected-entity .entity-label')
    expect(chip.text()).toBe('TKT-005')
    expect(chip.text()).not.toContain('object')
    wrapper.unmount()
  })

  it('dropdown items use the same "Title (ID)" / "ID" format', async () => {
    const titled = entity('TKT-100', 'Has a title')
    const untitled = entity('TKT-101')
    const wrapper = await mountPicker([], [titled, untitled])

    // Open the dropdown by focusing the search input.
    const search = wrapper.find('input[role="combobox"]')
    await search.trigger('focus')
    await flushPromises()

    const items = wrapper.findAll('.dropdown-item .entity-label')
    const texts = items.map((i) => i.text())
    expect(texts).toContain('Has a title (TKT-100)')
    expect(texts).toContain('TKT-101')
    wrapper.unmount()
  })
})
