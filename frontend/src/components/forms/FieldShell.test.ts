import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import FieldShell from './FieldShell.vue'

function shell(props: Record<string, unknown>) {
  return mount(FieldShell, {
    props,
    slots: { default: '<input class="widget" />' },
  })
}

describe('FieldShell', () => {
  it('renders label before the control by default', () => {
    const w = shell({ fieldId: 'f-title', label: 'Title' })
    const html = w.html()
    expect(html.indexOf('Title')).toBeLessThan(html.indexOf('class="widget"'))
    expect(w.find('label').attributes('for')).toBe('f-title')
    expect(w.find('.checkbox-wrapper').exists()).toBe(false)
  })

  it('renders label after the control when labelPosition is after', () => {
    const w = shell({ fieldId: 'f-done', label: 'Done', labelPosition: 'after' })
    expect(w.find('.checkbox-wrapper').exists()).toBe(true)
    const html = w.html()
    expect(html.indexOf('class="widget"')).toBeLessThan(html.indexOf('Done'))
  })

  it('shows a required asterisk only when required', () => {
    expect(shell({ label: 'X', required: true }).find('.required').exists()).toBe(true)
    expect(shell({ label: 'X' }).find('.required').exists()).toBe(false)
  })

  it('renders help and error text when provided', () => {
    const w = shell({ label: 'X', help: 'do this', error: 'bad' })
    expect(w.find('.field-help').text()).toBe('do this')
    expect(w.find('.field-error').text()).toBe('bad')
    expect(w.find('.form-field').classes()).toContain('has-error')
  })

  it('omits help/error when absent and is not in error state', () => {
    const w = shell({ label: 'X' })
    expect(w.find('.field-help').exists()).toBe(false)
    expect(w.find('.field-error').exists()).toBe(false)
    expect(w.find('.form-field').classes()).not.toContain('has-error')
  })

  it('omits the label element when no label is given', () => {
    expect(shell({}).find('label').exists()).toBe(false)
  })

  it('omits the label element in after-position when no label is given', () => {
    const w = shell({ labelPosition: 'after' })
    expect(w.find('.checkbox-wrapper label').exists()).toBe(false)
    expect(w.find('.widget').exists()).toBe(true)
  })

  it('omits the for attribute on the label when fieldId is undefined', () => {
    // Defensive shape check — happens whenever a field config arrives
    // without a property name. Harmless but must not blow up rendering.
    const w = shell({ label: 'X' })
    const label = w.find('label')
    expect(label.exists()).toBe(true)
    expect(label.attributes('for')).toBeUndefined()
  })

  /**
   * The help and error text used to render with no association to the input at
   * all: a screen-reader user heard the label and then nothing. The shell owns
   * the ids because it renders the text, but it cannot reach across the slot
   * boundary to set `aria-describedby` -- so it hands them to the control
   * instead, and these pin that handoff.
   */
  describe('accessibility wiring', () => {
    function described(props: Record<string, unknown>) {
      const w = mount(FieldShell, {
        props,
        slots: {
          default: `<template #default="c"><input class="widget" :aria-describedby="c.describedBy" :aria-invalid="c.invalid || undefined" /></template>`,
        },
      })
      return w
    }

    it('points the control at the help text', () => {
      const w = described({ label: 'X', help: 'do this' })
      const id = w.find('.field-help').attributes('id')
      expect(id).toBeTruthy()
      expect(w.find('.widget').attributes('aria-describedby')).toBe(id)
    })

    /** Error first: what just went wrong before the original guidance. */
    it('announces the error before the help when both are present', () => {
      const w = described({ label: 'X', help: 'do this', error: 'bad' })
      const helpId = w.find('.field-help').attributes('id')
      const errorId = w.find('.field-error').attributes('id')
      expect(w.find('.widget').attributes('aria-describedby')).toBe(`${errorId} ${helpId}`)
    })

    it('describes nothing when there is neither help nor error', () => {
      const w = described({ label: 'X' })
      expect(w.find('.widget').attributes('aria-describedby')).toBeUndefined()
    })

    it('marks the control invalid only while in error', () => {
      expect(described({ label: 'X', error: 'bad' }).find('.widget').attributes('aria-invalid')).toBe('true')
      expect(described({ label: 'X' }).find('.widget').attributes('aria-invalid')).toBeUndefined()
    })

    /**
     * A message that appears after the user has left the field has to announce
     * itself; otherwise they have to go back and look for it.
     */
    it('gives the error message an alert role', () => {
      const w = described({ label: 'X', error: 'bad' })
      expect(w.find('.field-error').attributes('role')).toBe('alert')
    })
  })
})
