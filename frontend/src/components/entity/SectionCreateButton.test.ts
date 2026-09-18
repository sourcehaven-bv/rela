// The section create affordance (TKT-R4BMJM, AC1 + AC2 client half).
//
// The server decides WHETHER to offer a target and WHICH targets: presence in
// `targets` is the permission answer. So these tests are about rendering that
// answer faithfully, and specifically about not inventing an affordance the
// server did not send — the detail page is read-only by default (TKT-651W) and
// this component is the only thing that can break that on the client.

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import SectionCreateButton from './SectionCreateButton.vue'
import type { ViewSectionCreate } from '@/api/views'

function affordance(overrides: Partial<ViewSectionCreate> = {}): ViewSectionCreate {
  return {
    relation: 'has-task',
    linkAs: 'to',
    peerId: 'EPIC-1',
    flow: 'modal',
    targets: [{ entityType: 'task', formId: 'create_task', label: 'Task' }],
    ...overrides,
  }
}

describe('SectionCreateButton', () => {
  it('renders nothing when the server sent no affordance', () => {
    const wrapper = mount(SectionCreateButton, { props: {} })

    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('renders nothing when the affordance has no targets', () => {
    // The server emits this shape for a principal who may create none of the
    // reachable types. An empty menu would advertise a capability they lack.
    const wrapper = mount(SectionCreateButton, {
      props: { create: affordance({ targets: [] }) },
    })

    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('renders a direct button for a single target, with no menu', () => {
    // The common case. A relation reaching one type should not cost a click to
    // choose between one option.
    const wrapper = mount(SectionCreateButton, { props: { create: affordance() } })

    const btn = wrapper.find('button.btn-section-create')
    expect(btn.exists()).toBe(true)
    expect(btn.text()).toContain('Task')
    expect(btn.attributes('aria-haspopup')).toBeUndefined()
  })

  it('emits the chosen target directly for a single target', async () => {
    const create = affordance()
    const wrapper = mount(SectionCreateButton, { props: { create } })

    await wrapper.find('button.btn-section-create').trigger('click')

    const emitted = wrapper.emitted('select')
    expect(emitted).toHaveLength(1)
    expect(emitted![0][0]).toEqual(create)
    expect(emitted![0][1]).toEqual(create.targets[0])
  })

  it('renders a menu for several targets and emits the one chosen', async () => {
    const create = affordance({
      targets: [
        { entityType: 'task', formId: 'create_task', label: 'Task' },
        { entityType: 'bug', formId: 'create_bug', label: 'Bug' },
      ],
    })
    const wrapper = mount(SectionCreateButton, { props: { create } })

    const toggle = wrapper.find('button.btn-section-create')
    expect(toggle.attributes('aria-haspopup')).toBe('menu')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(wrapper.find('.create-menu').exists()).toBe(false)

    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')

    const items = wrapper.findAll('.create-menu-item')
    expect(items.map((i) => i.text())).toEqual(['Task', 'Bug'])

    await items[1].trigger('click')

    const emitted = wrapper.emitted('select')
    expect(emitted).toHaveLength(1)
    expect(emitted![0][1]).toEqual(create.targets[1])
    // Choosing closes the menu; leaving it open over a modal would strand it.
    expect(wrapper.find('.create-menu').exists()).toBe(false)
  })

  it('carries the per-target template through to the host', async () => {
    // The server resolved `create.types.<type>.template`; the client must not
    // drop it, or the operator's preselection silently does nothing.
    const create = affordance({
      targets: [{ entityType: 'task', formId: 'create_task', label: 'Task', template: 'bugfix' }],
    })
    const wrapper = mount(SectionCreateButton, { props: { create } })

    await wrapper.find('button.btn-section-create').trigger('click')

    expect(wrapper.emitted('select')![0][1]).toMatchObject({ template: 'bugfix' })
  })
})
