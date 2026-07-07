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

// TKT-G7N5: RelationPicker consumes the per-relation-type affordance
// verdict. `creatable === false` hides the search input + inline-add
// button; `removable === false` hides every per-entity x.
describe('RelationPicker — affordance verdicts', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  async function mountWithVerdict(
    value: string[],
    candidates: Entity[],
    verdict: { creatable?: boolean; removable?: boolean }
  ) {
    seedSchema()
    seedCandidates(candidates)
    const field: FormFieldOrRelation = { relation: 'affects', label: 'Affects' }
    const wrapper = mount(RelationPicker, {
      props: { field, entityType: 'ticket', value, verdict },
      attachTo: document.body,
    })
    await flushPromises()
    return wrapper
  }

  it('default (no verdict): search input and per-entity x button both visible', async () => {
    const wrapper = await mountPicker([entity('TKT-100').id], [entity('TKT-100')])
    expect(wrapper.find('.search-wrapper').exists()).toBe(true)
    expect(wrapper.find('.remove-btn').exists()).toBe(true)
    wrapper.unmount()
  })

  it('creatable=false: search wrapper absent', async () => {
    const wrapper = await mountWithVerdict([entity('TKT-100').id], [entity('TKT-100')], {
      creatable: false,
    })
    expect(wrapper.find('.search-wrapper').exists()).toBe(false)
    // Removal still permitted.
    expect(wrapper.find('.remove-btn').exists()).toBe(true)
    wrapper.unmount()
  })

  it('removable=false: per-entity x absent on every selected entity', async () => {
    const a = entity('TKT-100')
    const b = entity('TKT-101')
    const wrapper = await mountWithVerdict([a.id, b.id], [a, b], {
      removable: false,
    })
    expect(wrapper.findAll('.remove-btn').length).toBe(0)
    // Search still available.
    expect(wrapper.find('.search-wrapper').exists()).toBe(true)
    wrapper.unmount()
  })

  it('both creatable=false and removable=false: both affordances hidden', async () => {
    const wrapper = await mountWithVerdict([entity('TKT-100').id], [entity('TKT-100')], {
      creatable: false,
      removable: false,
    })
    expect(wrapper.find('.search-wrapper').exists()).toBe(false)
    expect(wrapper.find('.remove-btn').exists()).toBe(false)
    wrapper.unmount()
  })
})

// BUG-10IPBP: on the CREATE form (no entityId) a `direction: incoming`
// picker used to silently drop every selection — loadIncomingValue()
// short-circuited without setting incomingLoaded, so emitIncomingDiff()
// no-op'd on each pick and nothing reached the create POST body. Edit
// mode worked because the entity existed and the snapshot loaded. The
// fix establishes an empty baseline in create mode so incoming picks
// emit as pure additions.
describe('RelationPicker — incoming direction on create (BUG-10IPBP)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  function seedIncomingSchema(maxIncoming = 10) {
    const schemaStore = useSchemaStore()
    schemaStore.entityTypes.set('ticket', {
      name: 'ticket',
      label: 'Ticket',
      properties: {},
    } as never)
    // An incoming picker selects sources from the relation's `from` set.
    // `max_incoming` drives single- vs multi-select on the picker.
    schemaStore.relationTypes.set('blocks', {
      name: 'blocks',
      from: ['ticket'],
      to: ['ticket'],
      inverse: 'blockedBy',
      max_outgoing: 10,
      max_incoming: maxIncoming,
    } as never)
  }

  async function mountIncoming(
    entityId: string | undefined,
    candidates: Entity[],
    maxIncoming = 10
  ) {
    seedIncomingSchema(maxIncoming)
    seedCandidates(candidates)
    const field: FormFieldOrRelation = {
      relation: 'blocks',
      label: 'Blocked By',
      direction: 'incoming',
    }
    const wrapper = mount(RelationPicker, {
      props: { field, entityType: 'ticket', entityId, value: [] },
      attachTo: document.body,
    })
    await flushPromises()
    return wrapper
  }

  it('create mode: selecting an incoming peer emits incoming-changed as an addition', async () => {
    const peer = entity('TKT-900', 'A blocker')
    // No entityId → create form.
    const wrapper = await mountIncoming(undefined, [peer])

    const search = wrapper.find('input[role="combobox"]')
    await search.trigger('focus')
    await flushPromises()
    await wrapper.find('.dropdown-item').trigger('click')
    await flushPromises()

    const events = wrapper.emitted('incoming-changed')
    expect(events).toBeTruthy()
    const payload = events![events!.length - 1][0] as {
      added: Array<{ targetId: string }>
      removed: string[]
      currentEntries: Array<{ id: string }>
    }
    expect(payload.added).toEqual([{ targetId: 'TKT-900' }])
    expect(payload.removed).toEqual([])
    expect(payload.currentEntries.map((e) => e.id)).toEqual(['TKT-900'])
    // The chip renders so the user sees the pending selection.
    expect(wrapper.find('.selected-entity').text()).toContain('TKT-900')
    wrapper.unmount()
  })

  it('create mode: removing a just-added incoming peer emits an empty desired set', async () => {
    const peer = entity('TKT-901')
    const wrapper = await mountIncoming(undefined, [peer])

    const search = wrapper.find('input[role="combobox"]')
    await search.trigger('focus')
    await flushPromises()
    await wrapper.find('.dropdown-item').trigger('click')
    await flushPromises()
    await wrapper.find('.remove-btn').trigger('click')
    await flushPromises()

    const events = wrapper.emitted('incoming-changed')!
    const last = events[events.length - 1][0] as {
      added: Array<{ targetId: string }>
      currentEntries: Array<{ id: string }>
    }
    expect(last.added).toEqual([])
    expect(last.currentEntries).toEqual([])
    wrapper.unmount()
  })

  it('create mode: an untouched incoming picker emits nothing', async () => {
    // The fix establishes an empty loaded baseline on create. That must
    // NOT translate into a spurious emit: a picker the user never touches
    // has to stay silent, or it would risk a `data: []` wipe in the body.
    const peer = entity('TKT-902')
    const wrapper = await mountIncoming(undefined, [peer])

    // No interaction at all — just mounted.
    expect(wrapper.emitted('incoming-changed')).toBeFalsy()
    wrapper.unmount()
  })

  it('create mode: single-select incoming picker (max_incoming=1) emits the addition', async () => {
    // The empty-baseline fix also feeds the single-select branch
    // (selectEntity replaces the list with [id] when !isMulti). Cover it
    // so single-cardinality incoming relations persist on create too.
    const peer = entity('TKT-903')
    const wrapper = await mountIncoming(undefined, [peer], 1)

    const search = wrapper.find('input[role="combobox"]')
    await search.trigger('focus')
    await flushPromises()
    await wrapper.find('.dropdown-item').trigger('click')
    await flushPromises()

    const events = wrapper.emitted('incoming-changed')
    expect(events).toBeTruthy()
    const payload = events![events!.length - 1][0] as {
      added: Array<{ targetId: string }>
      currentEntries: Array<{ id: string }>
    }
    expect(payload.added).toEqual([{ targetId: 'TKT-903' }])
    expect(payload.currentEntries.map((e) => e.id)).toEqual(['TKT-903'])
    wrapper.unmount()
  })
})
