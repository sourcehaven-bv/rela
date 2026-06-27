import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import HelpButton from './HelpButton.vue'

vi.mock('@/components/ui/HelpModal.vue', () => ({
  default: {
    name: 'HelpModal',
    props: ['open', 'entityType', 'entityLabel'],
    emits: ['close'],
    template: '<div data-testid="help-modal-stub" :data-open="open" :data-entity-type="entityType" />',
  },
}))

describe('HelpButton', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('renders a 44x44 button by class contract', () => {
    const w = mount(HelpButton)
    const btn = w.find('button.help-button')
    expect(btn.exists()).toBe(true)
  })

  it('uses the title prop for tooltip and aria-label', () => {
    const w = mount(HelpButton, { props: { title: 'Search syntax' } })
    const btn = w.find('button.help-button')
    expect(btn.attributes('title')).toBe('Search syntax')
    expect(btn.attributes('aria-label')).toBe('Search syntax')
  })

  it('falls back to "Show help" when no title is set', () => {
    const w = mount(HelpButton)
    const btn = w.find('button.help-button')
    expect(btn.attributes('title')).toBe('Show help')
  })

  it('toggles aria-expanded when clicked', async () => {
    const w = mount(HelpButton, {
      slots: { content: () => h('p', 'help body') },
      attachTo: document.body,
    })
    const btn = w.find('button.help-button')
    expect(btn.attributes('aria-expanded')).toBe('false')

    await btn.trigger('click')
    expect(btn.attributes('aria-expanded')).toBe('true')

    await btn.trigger('click')
    expect(btn.attributes('aria-expanded')).toBe('false')

    w.unmount()
  })

  it('renders content slot in a teleported modal when open and no entityType', async () => {
    const w = mount(HelpButton, {
      slots: { content: () => h('p', { class: 'slot-body' }, 'help body') },
      attachTo: document.body,
    })
    expect(document.querySelector('.help-button__modal')).toBeNull()

    await w.find('button.help-button').trigger('click')

    const modal = document.querySelector('.help-button__modal')
    expect(modal).not.toBeNull()
    expect(document.querySelector('.slot-body')?.textContent).toBe('help body')

    w.unmount()
  })

  it('delegates to HelpModal when entityType is provided', async () => {
    const w = mount(HelpButton, {
      props: { entityType: 'ticket', entityLabel: 'Ticket' },
    })
    const stub = w.find('[data-testid="help-modal-stub"]')
    expect(stub.exists()).toBe(true)
    expect(stub.attributes('data-entity-type')).toBe('ticket')
    expect(stub.attributes('data-open')).toBe('false')

    await w.find('button.help-button').trigger('click')
    expect(w.find('[data-testid="help-modal-stub"]').attributes('data-open')).toBe('true')
  })

  it('does not render the slot modal when entityType is set (delegates to HelpModal)', async () => {
    const w = mount(HelpButton, {
      props: { entityType: 'ticket' },
      slots: { content: () => h('p', 'should not show') },
      attachTo: document.body,
    })
    await w.find('button.help-button').trigger('click')

    expect(document.querySelector('.help-button__modal')).toBeNull()

    w.unmount()
  })
})
