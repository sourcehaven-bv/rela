import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import TextWidget from './TextWidget.vue'
import TextareaWidget from './TextareaWidget.vue'
import NumberWidget from './NumberWidget.vue'
import CheckboxWidget from './CheckboxWidget.vue'
import DateWidget from './DateWidget.vue'
import SelectWidget from './SelectWidget.vue'

describe('TextWidget', () => {
  it('renders the value and emits update:modelValue on input', async () => {
    const w = mount(TextWidget, { props: { modelValue: 'hello' } })
    const input = w.find('input[type="text"]')
    expect((input.element as HTMLInputElement).value).toBe('hello')
    await input.setValue('world')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['world'])
  })

  it('renders empty for null/undefined', () => {
    expect((mount(TextWidget, { props: { modelValue: null } }).find('input').element as HTMLInputElement).value).toBe('')
    expect(
      (mount(TextWidget, { props: { modelValue: undefined } }).find('input').element as HTMLInputElement).value
    ).toBe('')
  })

  it('honours disabled', () => {
    const w = mount(TextWidget, { props: { modelValue: 'x', disabled: true } })
    expect(w.find('input').attributes('disabled')).toBeDefined()
  })
})

describe('TextareaWidget', () => {
  it('renders the value and emits on input', async () => {
    const w = mount(TextareaWidget, { props: { modelValue: 'multi\nline' } })
    const ta = w.find('textarea')
    expect((ta.element as HTMLTextAreaElement).value).toBe('multi\nline')
    await ta.setValue('changed')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['changed'])
  })
})

describe('NumberWidget', () => {
  it('emits a parsed integer for numeric input', async () => {
    const w = mount(NumberWidget, { props: { modelValue: 1 } })
    await w.find('input').setValue('42')
    expect(w.emitted('update:modelValue')?.[0]).toEqual([42])
  })

  it('emits the raw string when input does not parse to an integer', async () => {
    const w = mount(NumberWidget, { props: { modelValue: 1 } })
    // A number input clears to '' for non-numeric content; parseInt('')
    // is NaN, so the handler emits the raw value — exercising the NaN
    // branch that preserves FieldRenderer's historical behaviour.
    await w.find('input').setValue('')
    expect(w.emitted('update:modelValue')?.[0]).toEqual([''])
  })
})

describe('CheckboxWidget', () => {
  it('reflects boolean true and string "true"', () => {
    expect((mount(CheckboxWidget, { props: { modelValue: true } }).find('input').element as HTMLInputElement).checked).toBe(true)
    expect((mount(CheckboxWidget, { props: { modelValue: 'true' } }).find('input').element as HTMLInputElement).checked).toBe(true)
    expect((mount(CheckboxWidget, { props: { modelValue: false } }).find('input').element as HTMLInputElement).checked).toBe(false)
  })

  it('emits the checked boolean on change', async () => {
    const w = mount(CheckboxWidget, { props: { modelValue: false } })
    await w.find('input').setValue(true)
    expect(w.emitted('update:modelValue')?.[0]).toEqual([true])
  })
})

describe('DateWidget', () => {
  it('renders a date input and emits on input', async () => {
    const w = mount(DateWidget, { props: { modelValue: '2026-05-29' } })
    const input = w.find('input[type="date"]')
    expect(input.exists()).toBe(true)
    expect((input.element as HTMLInputElement).value).toBe('2026-05-29')
    await input.setValue('2026-06-01')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['2026-06-01'])
  })
})

describe('SelectWidget', () => {
  const def = { type: 'enum' as const, values: ['open', 'review', 'done'] }

  it('renders options from propertyDef and emits the chosen value', async () => {
    const w = mount(SelectWidget, { props: { modelValue: 'open', propertyDef: def } })
    const opts = w.findAll('option').map((o) => o.attributes('value'))
    expect(opts).toEqual(['', 'open', 'review', 'done'])
    await w.find('select').setValue('review')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['review'])
  })

  it('disables options denied by optionVerdicts', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', propertyDef: def, optionVerdicts: { done: false } },
    })
    const byValue = Object.fromEntries(w.findAll('option').map((o) => [o.attributes('value'), o]))
    expect(byValue['done'].attributes('disabled')).toBeDefined()
    expect(byValue['review'].attributes('disabled')).toBeUndefined()
    expect(w.find('select').attributes('disabled')).toBeUndefined()
  })

  it('disables options not reachable by transition rules', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', propertyDef: def, transitions: { open: ['review'] } },
    })
    const byValue = Object.fromEntries(w.findAll('option').map((o) => [o.attributes('value'), o]))
    expect(byValue['done'].attributes('disabled')).toBeDefined()
    expect(byValue['review'].attributes('disabled')).toBeUndefined()
  })

  it('renders the transitions info panel when transitions are present', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', propertyDef: def, transitions: { open: ['review'] } },
    })
    expect(w.find('.transitions-info').exists()).toBe(true)
  })

  it('renders no transitions panel without transitions', () => {
    const w = mount(SelectWidget, { props: { modelValue: 'open', propertyDef: def } })
    expect(w.find('.transitions-info').exists()).toBe(false)
  })

  it('honours whole-select disabled', () => {
    const w = mount(SelectWidget, { props: { modelValue: 'open', propertyDef: def, disabled: true } })
    expect(w.find('select').attributes('disabled')).toBeDefined()
  })
})
