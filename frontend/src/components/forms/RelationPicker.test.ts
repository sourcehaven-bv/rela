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

// The server populates `_title` via metamodel.DisplayTitle: the configured
// display property's value if set, otherwise the entity id. The picker should
// render `${_title} (${id})` only when those differ.
function entity(id: string, displayTitle?: string): Entity {
  return {
    id,
    type: 'ticket',
    properties: {},
    _title: displayTitle ?? id,
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

  it('selected chip shows "<display title> (<id>)" when _title differs from id', async () => {
    const e = entity('TKT-001', 'Fix login bug')
    const wrapper = await mountPicker([e.id], [e])

    const chip = wrapper.find('.selected-entity .entity-label')
    expect(chip.exists()).toBe(true)
    expect(chip.text()).toBe('Fix login bug (TKT-001)')
    wrapper.unmount()
  })

  it('selected chip shows id alone when _title equals id (no display property set)', async () => {
    const e = entity('TKT-002')
    const wrapper = await mountPicker([e.id], [e])

    const chip = wrapper.find('.selected-entity .entity-label')
    expect(chip.text()).toBe('TKT-002')
    expect(chip.text()).not.toContain('(')
    wrapper.unmount()
  })

  it('selected chip shows id alone when _title is missing from the response', async () => {
    // Defensive: server should always populate _title, but if it does not we
    // must not render "undefined (id)".
    const e: Entity = { id: 'TKT-003', type: 'ticket', properties: {} }
    const wrapper = await mountPicker([e.id], [e])

    const chip = wrapper.find('.selected-entity .entity-label')
    expect(chip.text()).toBe('TKT-003')
    expect(chip.text()).not.toContain('undefined')
    wrapper.unmount()
  })

  it('dropdown items use the same "<display title> (<id>)" / "<id>" format', async () => {
    const titled = entity('TKT-100', 'Has a title')
    const untitled = entity('TKT-101')
    const wrapper = await mountPicker([], [titled, untitled])

    const search = wrapper.find('input[role="combobox"]')
    await search.trigger('focus')
    await flushPromises()

    const items = wrapper.findAll('.dropdown-item .entity-label')
    const texts = items.map((i) => i.text())
    expect(texts).toContain('Has a title (TKT-100)')
    expect(texts).toContain('TKT-101')
    wrapper.unmount()
  })

  it('dropdown search filters on _title (display name), not just id', async () => {
    const a = entity('TKT-200', 'Alpha feature')
    const b = entity('TKT-201', 'Beta feature')
    const wrapper = await mountPicker([], [a, b])

    const search = wrapper.find('input[role="combobox"]')
    await search.setValue('alpha')
    await flushPromises()

    const items = wrapper.findAll('.dropdown-item .entity-label')
    const texts = items.map((i) => i.text())
    expect(texts).toContain('Alpha feature (TKT-200)')
    expect(texts).not.toContain('Beta feature (TKT-201)')
    wrapper.unmount()
  })
})
