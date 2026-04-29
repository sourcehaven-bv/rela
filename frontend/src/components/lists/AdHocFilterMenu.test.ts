import { describe, it, expect, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import AdHocFilterMenu from './AdHocFilterMenu.vue'
import { useSchemaStore } from '@/stores/schema'

const ticketType = {
  name: 'ticket',
  label: 'Ticket',
  properties: {
    title: { type: 'string', values: null },
    status: { type: 'enum', values: ['open', 'closed'] },
    priority: { type: 'enum', values: ['low', 'high'] },
  },
}

function seedSchema() {
  const schema = useSchemaStore()
  schema.entityTypes.set('ticket', ticketType as never)
  return schema
}

describe('AdHocFilterMenu', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    seedSchema()
  })

  it('lists properties of the bound entity type and excludes locked ones', async () => {
    const wrapper = mount(AdHocFilterMenu, {
      props: {
        mode: 'list',
        entityType: ticketType as never,
        lockedProperties: new Set(['priority']),
      },
      attachTo: document.body,
    })
    await wrapper.find('.filter-btn').trigger('click')
    await flushPromises()

    const labels = wrapper.findAll('.option-label').map((n) => n.text())
    expect(labels).toContain('Title')
    expect(labels).toContain('Status')
    expect(labels).not.toContain('Priority')

    wrapper.unmount()
  })

  it('emits apply with property+value after picking an enum', async () => {
    const wrapper = mount(AdHocFilterMenu, {
      props: { mode: 'list', entityType: ticketType as never },
      attachTo: document.body,
    })
    await wrapper.find('.filter-btn').trigger('click')
    await flushPromises()

    const statusOption = wrapper
      .findAll('.filter-option')
      .find((n) => n.text().includes('Status'))
    expect(statusOption).toBeDefined()
    await statusOption!.trigger('click')

    const openOption = wrapper
      .findAll('.filter-option')
      .find((n) => n.text().trim() === 'open')
    expect(openOption).toBeDefined()
    await openOption!.trigger('click')

    const emitted = wrapper.emitted('apply')
    expect(emitted).toHaveLength(1)
    expect(emitted?.[0]).toEqual(['status', 'open'])

    wrapper.unmount()
  })

  it('emits apply with free-text value when property has no enum values', async () => {
    const wrapper = mount(AdHocFilterMenu, {
      props: { mode: 'list', entityType: ticketType as never },
      attachTo: document.body,
    })
    await wrapper.find('.filter-btn').trigger('click')
    await flushPromises()

    const titleOption = wrapper
      .findAll('.filter-option')
      .find((n) => n.text().includes('Title'))
    await titleOption!.trigger('click')
    await flushPromises()

    const valueInput = wrapper.find<HTMLInputElement>('.filter-text-input input')
    valueInput.element.value = 'foo'
    await valueInput.trigger('input')

    await wrapper.find('.btn-primary').trigger('click')

    const emitted = wrapper.emitted('apply')
    expect(emitted).toHaveLength(1)
    expect(emitted?.[0]).toEqual(['title', 'foo'])

    wrapper.unmount()
  })

  it('Escape returns to property picker, then closes the menu', async () => {
    const wrapper = mount(AdHocFilterMenu, {
      props: { mode: 'list', entityType: ticketType as never },
      attachTo: document.body,
    })
    await wrapper.find('.filter-btn').trigger('click')
    await flushPromises()

    const titleOption = wrapper
      .findAll('.filter-option')
      .find((n) => n.text().includes('Title'))
    await titleOption!.trigger('click')
    await flushPromises()

    // First Escape: drops back to property picker.
    await wrapper.find('.filter-menu').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('.filter-text-input').exists()).toBe(false)

    // Second Escape: closes the menu entirely.
    await wrapper.find('.filter-menu').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('.filter-menu').exists()).toBe(false)

    wrapper.unmount()
  })

  it('search mode includes a synthetic Type option', async () => {
    const wrapper = mount(AdHocFilterMenu, {
      props: { mode: 'search' },
      attachTo: document.body,
    })
    await wrapper.find('.filter-btn').trigger('click')
    await flushPromises()

    const labels = wrapper.findAll('.option-label').map((n) => n.text())
    expect(labels).toContain('Entity Type')

    wrapper.unmount()
  })

  it('list mode without entityType renders no options (does not fall back to all-types)', async () => {
    // Regression test for C4: previously `if (entityType) {...} else {fall
    // back to schema union}` produced surprising properties when the schema
    // store was still loading or the list config pointed at an unknown type.
    const wrapper = mount(AdHocFilterMenu, {
      props: { mode: 'list' },
      attachTo: document.body,
    })
    await wrapper.find('.filter-btn').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.option-label')).toHaveLength(0)
    expect(wrapper.find('.filter-empty').exists()).toBe(true)

    wrapper.unmount()
  })
})
