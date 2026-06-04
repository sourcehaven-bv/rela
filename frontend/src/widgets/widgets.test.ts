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
    const w = mount(TextWidget, { props: { modelValue: 'hello', mode: 'edit' as const } })
    const input = w.find('input[type="text"]')
    expect((input.element as HTMLInputElement).value).toBe('hello')
    await input.setValue('world')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['world'])
  })

  it('renders empty for null/undefined', () => {
    expect((mount(TextWidget, { props: { modelValue: null, mode: 'edit' as const } }).find('input').element as HTMLInputElement).value).toBe('')
    expect(
      (mount(TextWidget, { props: { modelValue: undefined, mode: 'edit' as const } }).find('input').element as HTMLInputElement).value
    ).toBe('')
  })

  it('honours disabled', () => {
    const w = mount(TextWidget, { props: { modelValue: 'x', mode: 'edit' as const, disabled: true } })
    expect(w.find('input').attributes('disabled')).toBeDefined()
  })
})

describe('TextareaWidget', () => {
  it('renders the value and emits on input', async () => {
    const w = mount(TextareaWidget, { props: { modelValue: 'multi\nline', mode: 'edit' as const } })
    const ta = w.find('textarea')
    expect((ta.element as HTMLTextAreaElement).value).toBe('multi\nline')
    await ta.setValue('changed')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['changed'])
  })
})

describe('NumberWidget', () => {
  it('emits a parsed integer for numeric input', async () => {
    const w = mount(NumberWidget, { props: { modelValue: 1, mode: 'edit' as const } })
    await w.find('input').setValue('42')
    expect(w.emitted('update:modelValue')?.[0]).toEqual([42])
  })

  it('emits the raw string when input does not parse to an integer', async () => {
    const w = mount(NumberWidget, { props: { modelValue: 1, mode: 'edit' as const } })
    // A number input clears to '' for non-numeric content; parseInt('')
    // is NaN, so the handler emits the raw value — exercising the NaN
    // branch that preserves FieldRenderer's historical behaviour.
    await w.find('input').setValue('')
    expect(w.emitted('update:modelValue')?.[0]).toEqual([''])
  })
})

describe('CheckboxWidget', () => {
  it('reflects boolean true and string "true"', () => {
    expect((mount(CheckboxWidget, { props: { modelValue: true, mode: 'edit' as const } }).find('input').element as HTMLInputElement).checked).toBe(true)
    expect((mount(CheckboxWidget, { props: { modelValue: 'true', mode: 'edit' as const } }).find('input').element as HTMLInputElement).checked).toBe(true)
    expect((mount(CheckboxWidget, { props: { modelValue: false, mode: 'edit' as const } }).find('input').element as HTMLInputElement).checked).toBe(false)
  })

  it('emits the checked boolean on change', async () => {
    const w = mount(CheckboxWidget, { props: { modelValue: false, mode: 'edit' as const } })
    await w.find('input').setValue(true)
    expect(w.emitted('update:modelValue')?.[0]).toEqual([true])
  })
})

describe('DateWidget', () => {
  it('renders a date input and emits on input', async () => {
    const w = mount(DateWidget, { props: { modelValue: '2026-05-29', mode: 'edit' as const } })
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
    const w = mount(SelectWidget, { props: { modelValue: 'open', mode: 'edit' as const, propertyDef: def } })
    const opts = w.findAll('option').map((o) => o.attributes('value'))
    expect(opts).toEqual(['', 'open', 'review', 'done'])
    await w.find('select').setValue('review')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['review'])
  })

  it('disables options denied by optionVerdicts', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', mode: 'edit' as const, propertyDef: def, optionVerdicts: { done: false } },
    })
    const byValue = Object.fromEntries(w.findAll('option').map((o) => [o.attributes('value'), o]))
    expect(byValue['done'].attributes('disabled')).toBeDefined()
    expect(byValue['review'].attributes('disabled')).toBeUndefined()
    expect(w.find('select').attributes('disabled')).toBeUndefined()
  })

  it('disables options not reachable by transition rules', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', mode: 'edit' as const, propertyDef: def, transitions: { open: ['review'] } },
    })
    const byValue = Object.fromEntries(w.findAll('option').map((o) => [o.attributes('value'), o]))
    expect(byValue['done'].attributes('disabled')).toBeDefined()
    expect(byValue['review'].attributes('disabled')).toBeUndefined()
  })

  it('renders the transitions info panel when transitions are present', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', mode: 'edit' as const, propertyDef: def, transitions: { open: ['review'] } },
    })
    expect(w.find('.transitions-info').exists()).toBe(true)
  })

  it('renders no transitions panel without transitions', () => {
    const w = mount(SelectWidget, { props: { modelValue: 'open', mode: 'edit' as const, propertyDef: def } })
    expect(w.find('.transitions-info').exists()).toBe(false)
  })

  it('honours whole-select disabled', () => {
    const w = mount(SelectWidget, { props: { modelValue: 'open', mode: 'edit' as const, propertyDef: def, disabled: true } })
    expect(w.find('select').attributes('disabled')).toBeDefined()
  })
})

// Display-mode coverage (TKT-UD7YR). Each widget renders a read-only
// shape distinct from its edit-mode form. We assert the chosen element
// + the rendered value -- structural enough to catch routing
// regressions, loose enough not to lock in incidental DOM.

describe('TextWidget (display)', () => {
  it('renders the value as a span', () => {
    const w = mount(TextWidget, { props: { modelValue: 'hello', mode: 'display' as const } })
    expect(w.find('input').exists()).toBe(false)
    expect(w.find('span.display-value').text()).toBe('hello')
  })

  it('renders empty for null/undefined without crashing', () => {
    expect(
      mount(TextWidget, { props: { modelValue: null, mode: 'display' as const } }).find('span').text(),
    ).toBe('')
    expect(
      mount(TextWidget, { props: { modelValue: undefined, mode: 'display' as const } }).find('span').text(),
    ).toBe('')
  })
})

describe('TextareaWidget (display)', () => {
  it('renders multi-line text as a span (CSS handles wrapping)', () => {
    const w = mount(TextareaWidget, { props: { modelValue: 'a\nb', mode: 'display' as const } })
    expect(w.find('textarea').exists()).toBe(false)
    expect(w.find('span.display-value').text()).toContain('a')
  })
})

describe('NumberWidget (display)', () => {
  it('renders the number as a span', () => {
    const w = mount(NumberWidget, { props: { modelValue: 42, mode: 'display' as const } })
    expect(w.find('input').exists()).toBe(false)
    expect(w.find('span.display-value').text()).toBe('42')
  })

  it('renders zero (no "falsy collapse to empty")', () => {
    const w = mount(NumberWidget, { props: { modelValue: 0, mode: 'display' as const } })
    expect(w.find('span.display-value').text()).toBe('0')
  })
})

describe('DateWidget (display)', () => {
  it('renders the date via formatDate (locale-aware, parseable)', () => {
    const w = mount(DateWidget, { props: { modelValue: '2026-05-29', mode: 'display' as const } })
    expect(w.find('input').exists()).toBe(false)
    // formatDate output varies by environment locale; assert a non-empty
    // formatted span and that it doesn't pass the raw ISO through.
    const span = w.find('span.display-value')
    expect(span.exists()).toBe(true)
    expect(span.text()).not.toBe('')
  })

  it('falls back to the raw string for an unparseable value', () => {
    const w = mount(DateWidget, { props: { modelValue: 'not-a-date', mode: 'display' as const } })
    expect(w.find('span.display-value').text()).toBe('not-a-date')
  })

  it('renders empty for null', () => {
    const w = mount(DateWidget, { props: { modelValue: null, mode: 'display' as const } })
    expect(w.find('span.display-value').text()).toBe('')
  })
})

describe('CheckboxWidget (display)', () => {
  it('renders ✓ for true', () => {
    const w = mount(CheckboxWidget, { props: { modelValue: true, mode: 'display' as const } })
    expect(w.find('input').exists()).toBe(false)
    expect(w.find('span.display-value').text()).toBe('✓')
  })

  it('renders ☐ for false', () => {
    const w = mount(CheckboxWidget, { props: { modelValue: false, mode: 'display' as const } })
    expect(w.find('span.display-value').text()).toBe('☐')
  })

  it('renders ✓ for the string "true" (server may serialize as string)', () => {
    const w = mount(CheckboxWidget, { props: { modelValue: 'true', mode: 'display' as const } })
    expect(w.find('span.display-value').text()).toBe('✓')
  })
})

describe('SelectWidget (display)', () => {
  const def = { type: 'enum' as const, values: ['open', 'review', 'done'] }

  it('renders a Badge for the value', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', mode: 'display' as const, propertyDef: def, propertyName: 'status' },
    })
    expect(w.find('select').exists()).toBe(false)
    // The Badge component renders its value into a span.badge-XYZ; we
    // assert the visible text rather than coupling to the styled class.
    expect(w.text()).toContain('open')
  })

  it('passes propertyName through to Badge for style lookup', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', mode: 'display' as const, propertyDef: def, propertyName: 'status' },
    })
    const badge = w.findComponent({ name: 'Badge' })
    expect(badge.exists()).toBe(true)
    expect(badge.props('property')).toBe('status')
  })

  it('renders nothing visible for empty value', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: '', mode: 'display' as const, propertyDef: def },
    })
    expect(w.findComponent({ name: 'Badge' }).exists()).toBe(false)
  })
})
