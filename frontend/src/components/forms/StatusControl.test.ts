// TKT-3G93B8: StatusControl renders ONLY the server-resolved allowed moves,
// labels them as transitions (action), and commits the target on select.

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import StatusControl from './StatusControl.vue'
import type { TransitionOption } from '@/types'

function renderControl(opts: {
  modelValue: string
  transitions: TransitionOption[]
  disabled?: boolean
}) {
  return mount(StatusControl, {
    props: {
      modelValue: opts.modelValue,
      property: 'status',
      entityType: 'ticket',
      transitions: opts.transitions,
      disabled: opts.disabled,
    },
    attachTo: document.body,
  })
}

describe('StatusControl', () => {
  it('shows only allowed moves, labeled by their action label', async () => {
    const wrapper = renderControl({
      modelValue: 'todo',
      transitions: [
        { to: 'doing', label: 'Start progress', allowed: true },
        { to: 'done', label: 'Complete', guard: 'close', allowed: false, reason: 'guard' },
      ],
    })
    // Open the menu.
    await wrapper.find('.status-trigger').trigger('click')

    const moves = wrapper.findAll('.status-move')
    expect(moves).toHaveLength(1) // only the allowed one
    expect(moves[0].text()).toContain('Start progress')
    // The non-performable 'Complete' move is not rendered at all.
    expect(wrapper.text()).not.toContain('Complete')
    wrapper.unmount()
  })

  it('falls back to the raw target value when no action label is set', async () => {
    const wrapper = renderControl({
      modelValue: 'todo',
      transitions: [{ to: 'doing', allowed: true }],
    })
    await wrapper.find('.status-trigger').trigger('click')
    // No label and no schema-store enum label configured → raw value.
    expect(wrapper.find('.status-move-label').text()).toBe('doing')
    wrapper.unmount()
  })

  it('emits update:modelValue with the target when a move is selected', async () => {
    const wrapper = renderControl({
      modelValue: 'todo',
      transitions: [{ to: 'doing', label: 'Start progress', allowed: true }],
    })
    await wrapper.find('.status-trigger').trigger('click')
    await wrapper.find('.status-move').trigger('click')

    const emitted = wrapper.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    expect(emitted![0]).toEqual(['doing'])
    wrapper.unmount()
  })

  it('renders no picker in a terminal state (no allowed moves)', () => {
    const wrapper = renderControl({
      modelValue: 'done',
      transitions: [], // terminal
    })
    // The trigger renders (showing the current state) but is inert.
    const trigger = wrapper.find('.status-trigger')
    expect(trigger.exists()).toBe(true)
    expect(trigger.attributes('disabled')).toBeDefined()
    // No caret, no menu.
    expect(wrapper.find('.status-caret').exists()).toBe(false)
    expect(wrapper.find('.status-menu').exists()).toBe(false)
    wrapper.unmount()
  })

  it('excludes a self-loop move even if the server sent one', async () => {
    const wrapper = renderControl({
      modelValue: 'doing',
      transitions: [
        { to: 'doing', label: 'Re-open', allowed: true }, // self-loop
        { to: 'done', label: 'Complete', allowed: true },
      ],
    })
    await wrapper.find('.status-trigger').trigger('click')
    const moves = wrapper.findAll('.status-move')
    expect(moves).toHaveLength(1)
    expect(moves[0].text()).toContain('Complete')
    wrapper.unmount()
  })

  it('does not open when disabled', async () => {
    const wrapper = renderControl({
      modelValue: 'todo',
      transitions: [{ to: 'doing', label: 'Start progress', allowed: true }],
      disabled: true,
    })
    await wrapper.find('.status-trigger').trigger('click')
    expect(wrapper.find('.status-menu').exists()).toBe(false)
    wrapper.unmount()
  })
})
