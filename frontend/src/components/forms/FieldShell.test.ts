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
})
