import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import RelationPicker from './RelationPicker.vue'
import RelationCards from './RelationCards.vue'
import { useSchemaStore } from '@/stores/schema'
import { useEntitiesStore } from '@/stores/entities'
import type { Entity } from '@/types'
import type { FormFieldOrRelation } from '@/types/config'

/**
 * Inline creation from a relation field (TKT-OMUD56), at the widget level.
 *
 * The two widgets differ in where a created entity lands: RelationPicker
 * selects it outright, while RelationCards routes it into `selectTarget` so
 * required edge properties can still be filled before linking.
 *
 * DynamicForm is stubbed here — mounting the real 1300-line form inside a modal
 * inside a widget would make these tests about the form, not the affordance.
 * The create→link flow through the real form is covered end-to-end in e2e.
 */

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return { ...actual, searchEntities: vi.fn().mockResolvedValue({ data: [] }) }
})

function seedSchema() {
  const schemaStore = useSchemaStore()
  schemaStore.entityTypes.set('ticket', { name: 'ticket', label: 'Ticket', properties: {} } as never)
  schemaStore.entityTypes.set('feature', {
    name: 'feature',
    label: 'Feature',
    properties: {},
  } as never)
  schemaStore.relationTypes.set('implements', {
    name: 'implements',
    from: ['ticket'],
    to: ['feature'],
  } as never)
  return schemaStore
}

function seedCandidates(entities: Entity[] = []) {
  useEntitiesStore().fetchList = vi.fn().mockResolvedValue({
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 100, has_more: false },
    included: {},
  })
}

const newFeature: Entity = {
  id: 'FEAT-9',
  type: 'feature',
  properties: {},
  _title: 'Inline created',
}

async function mountPicker() {
  const field: FormFieldOrRelation = { relation: 'implements', label: 'Implements' }
  const wrapper = mount(RelationPicker, {
    props: { field, entityType: 'ticket', value: [] },
    attachTo: document.body,
    global: { stubs: { InlineCreateFormModal: true } },
  })
  await flushPromises()
  return wrapper
}

async function mountCards() {
  const field: FormFieldOrRelation = {
    relation: 'implements',
    label: 'Implements',
    widget: 'cards',
  }
  const wrapper = mount(RelationCards, {
    props: { field, entityType: 'ticket', entityId: 'TKT-1' },
    attachTo: document.body,
    global: { stubs: { InlineCreateFormModal: true } },
  })
  await flushPromises()
  return wrapper
}

describe('RelationPicker — inline create affordance', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    seedSchema()
    seedCandidates()
  })

  it('offers "+ New <Label>" for an eligible target type', async () => {
    useSchemaStore().setInlineCreate({ feature: 'create_feature' })
    const wrapper = await mountPicker()

    await wrapper.find('input[role="combobox"]').trigger('focus')

    const buttons = wrapper.findAll('.add-new-btn')
    expect(buttons).toHaveLength(1)
    expect(buttons[0].text()).toBe('+ New Feature')
    wrapper.unmount()
  })

  it('offers nothing when the server did not list the target type', async () => {
    // AC2/AC3 at the widget: no create permission, or no form — either way the
    // type is absent from the payload and no button renders.
    useSchemaStore().setInlineCreate({})
    const wrapper = await mountPicker()

    await wrapper.find('input[role="combobox"]').trigger('focus')

    expect(wrapper.findAll('.add-new-btn')).toHaveLength(0)
    wrapper.unmount()
  })

  it('selects an inline-created entity and emits it as a relation value', async () => {
    // AC5: the created entity must be selectable even though it is not in the
    // pre-loaded candidate window.
    useSchemaStore().setInlineCreate({ feature: 'create_feature' })
    const wrapper = await mountPicker()

    await wrapper.find('input[role="combobox"]').trigger('focus')
    // Open via the button so the modal gets the form id the widget resolved,
    // exactly as a user reaches it.
    await wrapper.find('.add-new-btn').trigger('click')
    await wrapper.findComponent({ name: 'InlineCreateFormModal' }).vm.$emit('created', newFeature)
    await flushPromises()

    const updates = wrapper.emitted('update') ?? []
    expect(updates[updates.length - 1]?.[0]).toEqual(['FEAT-9'])
    // The widget owns the modal's lifecycle: the modal reports the entity and
    // does not close itself, so a picker that forgot this would leave the
    // dialog covering the form it just populated. Closing also clears the form
    // id, which unmounts the dialog rather than leaving it hidden-but-alive.
    expect(wrapper.findComponent({ name: 'InlineCreateFormModal' }).exists()).toBe(false)
    // The type map is what the submit-time PATCH builder needs to address the
    // edge; a created entity must populate it like any other selection.
    const typeUpdates = wrapper.emitted('update:types') ?? []
    expect(typeUpdates[typeUpdates.length - 1]?.[0]).toEqual(new Map([['FEAT-9', 'feature']]))
    wrapper.unmount()
  })
})

describe('RelationCards — inline create affordance', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    seedSchema()
  })

  it('offers "+ New <Label>" inside the add-search panel', async () => {
    useSchemaStore().setInlineCreate({ feature: 'create_feature' })
    const wrapper = await mountCards()

    await wrapper.find('.add-btn').trigger('click')

    const buttons = wrapper.findAll('.add-new-btn')
    expect(buttons).toHaveLength(1)
    expect(buttons[0].text()).toBe('+ New Feature')
    wrapper.unmount()
  })

  it('offers nothing when the target type is not listed', async () => {
    useSchemaStore().setInlineCreate({})
    const wrapper = await mountCards()

    await wrapper.find('.add-btn').trigger('click')

    expect(wrapper.findAll('.add-new-btn')).toHaveLength(0)
    wrapper.unmount()
  })

  it('routes a created entity into the link step rather than linking it outright', async () => {
    // AC5 for cards. Linking immediately would skip required edge properties,
    // so the created entity becomes the *selected target* and the user still
    // fills meta and presses Link.
    useSchemaStore().setInlineCreate({ feature: 'create_feature' })
    const wrapper = await mountCards()

    await wrapper.find('.add-btn').trigger('click')
    await wrapper.find('.add-new-btn').trigger('click')
    await wrapper.findComponent({ name: 'InlineCreateFormModal' }).vm.$emit('created', newFeature)
    await flushPromises()

    expect(wrapper.find('.new-relation-form').exists()).toBe(true)
    expect(wrapper.find('.selected-target').text()).toContain('FEAT-9')
    // Not linked yet: no edge has been emitted.
    expect(wrapper.emitted('cards-changed')).toBeUndefined()
    wrapper.unmount()
  })

  it('clears the created notice when a different target is picked', async () => {
    // Regression: the notice was cleared on link/cancel but not on re-select,
    // so "Created <X>" stayed pinned next to an unrelated entity Y.
    useSchemaStore().setInlineCreate({ feature: 'create_feature' })
    const wrapper = await mountCards()

    await wrapper.find('.add-btn').trigger('click')
    await wrapper.find('.add-new-btn').trigger('click')
    await wrapper.findComponent({ name: 'InlineCreateFormModal' }).vm.$emit('created', newFeature)
    await flushPromises()
    expect(wrapper.find('.created-notice').exists()).toBe(true)

    // Pick a different, pre-existing target.
    const other: Entity = { id: 'FEAT-2', type: 'feature', properties: {}, _title: 'Pre-existing' }
    ;(wrapper.vm as unknown as { selectTarget: (e: Entity) => void }).selectTarget(other)
    await flushPromises()

    expect(wrapper.find('.created-notice').exists()).toBe(false)
    expect(wrapper.find('.selected-target').text()).toContain('FEAT-2')
    wrapper.unmount()
  })

  it('says the entity was created so abandoning the link is not silent', async () => {
    // RR-3UOH1I: the entity is already persisted. Cancelling the link step
    // leaves it orphaned, which must not look like nothing happened.
    useSchemaStore().setInlineCreate({ feature: 'create_feature' })
    const wrapper = await mountCards()

    await wrapper.find('.add-btn').trigger('click')
    await wrapper.find('.add-new-btn').trigger('click')
    await wrapper.findComponent({ name: 'InlineCreateFormModal' }).vm.$emit('created', newFeature)
    await flushPromises()

    const notice = wrapper.find('.created-notice')
    expect(notice.exists()).toBe(true)
    expect(notice.text()).toContain('Inline created')
    wrapper.unmount()
  })
})
