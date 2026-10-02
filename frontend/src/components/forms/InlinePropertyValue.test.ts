// InlinePropertyValue picks how a value is changed in place (pick, live,
// swap, status) and emits `update` only for a change the user kept.

import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick, type Component } from 'vue'
import { setActivePinia, createPinia } from 'pinia'
import InlinePropertyValue from './InlinePropertyValue.vue'
import SelectWidget from '@/widgets/SelectWidget.vue'
import MultiSelectWidget from '@/widgets/MultiSelectWidget.vue'
import TextWidget from '@/widgets/TextWidget.vue'
import CheckboxWidget from '@/widgets/CheckboxWidget.vue'
import type { PropertyDef } from '@/types'

const ENUM_DEF = { type: 'enum', values: ['open', 'closed'] } as PropertyDef
const TEXT_DEF = { type: 'string' } as PropertyDef

function mountValue(props: {
  widget: Component
  value: unknown
  writable?: boolean
  propertyDef?: PropertyDef
  optionVerdicts?: Record<string, boolean>
  transitions?: { to: string; label: string; allowed: boolean }[]
  error?: string
}) {
  return mount(InlinePropertyValue, {
    props: {
      property: 'status',
      label: 'Status',
      entityType: 'ticket',
      entityId: 'TKT-001',
      writable: true,
      ...props,
    },
    attachTo: document.body,
  })
}

describe('InlinePropertyValue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders only the display widget when not writable', () => {
    const wrapper = mountValue({
      widget: TextWidget,
      value: 'x',
      writable: false,
      propertyDef: TEXT_DEF,
    })
    expect(wrapper.findComponent({ name: 'TextWidget' }).props('mode')).toBe('display')
    expect(wrapper.findComponent({ name: 'RlInlineEdit' }).exists()).toBe(false)
  })

  describe('pick (enum)', () => {
    it('offers a None option when the property may be empty', () => {
      const wrapper = mountValue({ widget: SelectWidget, value: 'open', propertyDef: ENUM_DEF })
      expect(wrapper.findComponent({ name: 'RlOptionSelect' }).props('options')).toEqual([
        '',
        'open',
        'closed',
      ])
    })

    it('offers no None option on a required property', () => {
      const def = { ...ENUM_DEF, required: true } as PropertyDef
      const wrapper = mountValue({ widget: SelectWidget, value: 'open', propertyDef: def })
      expect(wrapper.findComponent({ name: 'RlOptionSelect' }).props('options')).toEqual([
        'open',
        'closed',
      ])
    })

    it('emits the picked value, and undefined for None', async () => {
      const wrapper = mountValue({ widget: SelectWidget, value: 'open', propertyDef: ENUM_DEF })
      const select = wrapper.findComponent({ name: 'RlOptionSelect' })
      select.vm.$emit('update:modelValue', 'closed')
      select.vm.$emit('update:modelValue', '')
      expect(wrapper.emitted('update')).toEqual([['closed'], [undefined]])
    })

    it('does not emit when the current value is picked again', () => {
      const wrapper = mountValue({ widget: SelectWidget, value: 'open', propertyDef: ENUM_DEF })
      wrapper.findComponent({ name: 'RlOptionSelect' }).vm.$emit('update:modelValue', 'open')
      expect(wrapper.emitted('update')).toBeUndefined()
    })

    it('disables an option the server denied, never None', () => {
      const wrapper = mountValue({
        widget: SelectWidget,
        value: 'open',
        propertyDef: ENUM_DEF,
        optionVerdicts: { closed: false },
      })
      const isDisabled = wrapper
        .findComponent({ name: 'RlOptionSelect' })
        .props('isOptionDisabled') as (v: string) => boolean
      expect(isDisabled('closed')).toBe(true)
      expect(isDisabled('open')).toBe(false)
      expect(isDisabled('')).toBe(false)
    })
  })

  describe('multi (enum list)', () => {
    const TAGS_DEF = { type: 'enum', list: true, values: ['a', 'b', 'c'] } as PropertyDef

    it('is a live inline multi-select of the fixed values', () => {
      const wrapper = mountValue({ widget: MultiSelectWidget, value: ['a'], propertyDef: TAGS_DEF })
      const select = wrapper.findComponent({ name: 'RlMultiSelect' })
      expect(select.props('variant')).toBe('inline')
      expect(select.props('modelValue')).toEqual(['a'])
      expect((select.props('options') as { value: string }[]).map((o) => o.value)).toEqual([
        'a',
        'b',
        'c',
      ])
    })

    it('emits each toggle', () => {
      const wrapper = mountValue({ widget: MultiSelectWidget, value: ['a'], propertyDef: TAGS_DEF })
      wrapper.findComponent({ name: 'RlMultiSelect' }).vm.$emit('update:modelValue', ['a', 'b'])
      expect(wrapper.emitted('update')).toEqual([[['a', 'b']]])
    })

    it('disables a value the server denied', () => {
      const wrapper = mountValue({
        widget: MultiSelectWidget,
        value: [],
        propertyDef: TAGS_DEF,
        optionVerdicts: { b: false },
      })
      const isDisabled = wrapper
        .findComponent({ name: 'RlMultiSelect' })
        .props('isOptionDisabled') as (v: string) => boolean
      expect(isDisabled('b')).toBe(true)
      expect(isDisabled('a')).toBe(false)
    })

    it('keeps a free-text list as a swap', () => {
      const wrapper = mountValue({
        widget: MultiSelectWidget,
        value: ['x'],
        propertyDef: { type: 'string', list: true } as PropertyDef,
      })
      expect(wrapper.findComponent({ name: 'RlMultiSelect' }).exists()).toBe(false)
      expect(wrapper.findComponent({ name: 'RlInlineEdit' }).exists()).toBe(true)
    })
  })

  it('renders a state-machine field as its StatusControl', () => {
    const wrapper = mountValue({
      widget: SelectWidget,
      value: 'open',
      propertyDef: ENUM_DEF,
      transitions: [{ to: 'closed', label: 'Close', allowed: true }],
    })
    expect(wrapper.findComponent({ name: 'StatusControl' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'RlOptionSelect' }).exists()).toBe(false)
  })

  it('renders a checkbox live, saving each toggle', () => {
    const wrapper = mountValue({
      widget: CheckboxWidget,
      value: false,
      propertyDef: { type: 'boolean' } as PropertyDef,
    })
    const box = wrapper.findComponent({ name: 'CheckboxWidget' })
    expect(box.props('mode')).toBe('edit')
    box.vm.$emit('update:modelValue', true)
    expect(wrapper.emitted('update')).toEqual([[true]])
  })

  describe('swap (everything else)', () => {
    async function openEdit(wrapper: ReturnType<typeof mountValue>) {
      await wrapper.find('button').trigger('click')
      await nextTick()
      return wrapper.findComponent({ name: 'TextWidget' })
    }

    it('reads as the display widget until clicked', () => {
      const wrapper = mountValue({ widget: TextWidget, value: 'a', propertyDef: TEXT_DEF })
      expect(wrapper.findComponent({ name: 'TextWidget' }).props('mode')).toBe('display')
    })

    it('edits a draft and emits it only when kept', async () => {
      const wrapper = mountValue({ widget: TextWidget, value: 'a', propertyDef: TEXT_DEF })
      const input = await openEdit(wrapper)
      expect(input.props('mode')).toBe('edit')
      expect(input.props('modelValue')).toBe('a')
      input.vm.$emit('update:modelValue', 'b')
      await nextTick()
      expect(wrapper.emitted('update')).toBeUndefined()
      await wrapper.find('input').trigger('keydown', { key: 'Enter' })
      expect(wrapper.emitted('update')).toEqual([['b']])
    })

    it('does not emit a kept edit that changed nothing', async () => {
      const wrapper = mountValue({ widget: TextWidget, value: 'a', propertyDef: TEXT_DEF })
      await openEdit(wrapper)
      await wrapper.find('input').trigger('keydown', { key: 'Enter' })
      expect(wrapper.emitted('update')).toBeUndefined()
    })
  })

  it('shows an error under the value', () => {
    const wrapper = mountValue({ widget: TextWidget, value: 'a', propertyDef: TEXT_DEF })
    expect(wrapper.find('.inline-property-error').exists()).toBe(false)
    const withError = mountValue({
      widget: TextWidget,
      value: 'a',
      propertyDef: TEXT_DEF,
      error: 'invalid value',
    })
    expect(withError.find('.inline-property-error').text()).toBe('invalid value')
  })
})
