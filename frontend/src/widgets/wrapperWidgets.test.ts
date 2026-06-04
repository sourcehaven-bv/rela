import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import MultiSelectWidget from './MultiSelectWidget.vue'
import RruleWidget from './RruleWidget.vue'

// TagSelect wraps SlimSelect, whose MutationObserver doesn't mount
// cleanly under happy-dom; RruleBuilder is heavy. Both widgets are thin
// adapters, so we stub the wrapped component and assert the adapter's
// prop mapping and event re-emission instead.

describe('MultiSelectWidget', () => {
  const stubs = { TagSelect: true }

  it('maps an array model value to string options for TagSelect', () => {
    const w = mount(MultiSelectWidget, {
      props: { modelValue: ['a', 1], mode: 'edit' as const, propertyDef: { type: 'enum', values: ['a', '1', 'b'] } },
      global: { stubs },
    })
    const tag = w.findComponent({ name: 'TagSelect' })
    expect(tag.props('modelValue')).toEqual(['a', '1'])
    expect(tag.props('options')).toEqual(['a', '1', 'b'])
  })

  it('coerces a scalar model value to a single-item array', () => {
    const w = mount(MultiSelectWidget, {
      props: { modelValue: 'solo', mode: 'edit' as const, propertyDef: { type: 'enum' } },
      global: { stubs },
    })
    expect(w.findComponent({ name: 'TagSelect' }).props('modelValue')).toEqual(['solo'])
  })

  it('treats empty model value as an empty array', () => {
    const w = mount(MultiSelectWidget, {
      props: { modelValue: null, mode: 'edit' as const, propertyDef: { type: 'enum' } },
      global: { stubs },
    })
    expect(w.findComponent({ name: 'TagSelect' }).props('modelValue')).toEqual([])
  })

  it('re-emits TagSelect updates as update:modelValue', async () => {
    const w = mount(MultiSelectWidget, {
      props: { modelValue: [], mode: 'edit' as const, propertyDef: { type: 'enum' } },
      global: { stubs },
    })
    w.findComponent({ name: 'TagSelect' }).vm.$emit('update:modelValue', ['x', 'y'])
    await w.vm.$nextTick()
    expect(w.emitted('update:modelValue')?.[0]).toEqual([['x', 'y']])
  })

  it('passes disabled and optionVerdicts through', () => {
    const w = mount(MultiSelectWidget, {
      props: {
        modelValue: [],
        mode: 'edit' as const,
        propertyDef: { type: 'enum', values: ['a'] },
        disabled: true,
        optionVerdicts: { a: false },
      },
      global: { stubs },
    })
    const tag = w.findComponent({ name: 'TagSelect' })
    expect(tag.props('disabled')).toBe(true)
    expect(tag.props('optionVerdicts')).toEqual({ a: false })
  })
})

describe('RruleWidget', () => {
  const stubs = { RruleBuilder: true }

  it('passes the stringified model value and help to RruleBuilder', () => {
    const w = mount(RruleWidget, {
      props: { modelValue: 'FREQ=DAILY', mode: 'edit' as const, help: 'every day', propertyDef: { type: 'rrule' } },
      global: { stubs },
    })
    const builder = w.findComponent({ name: 'RruleBuilder' })
    expect(builder.props('modelValue')).toBe('FREQ=DAILY')
    expect(builder.props('help')).toBe('every day')
  })

  it('renders empty for a null model value', () => {
    const w = mount(RruleWidget, {
      props: { modelValue: null, mode: 'edit' as const, propertyDef: { type: 'rrule' } },
      global: { stubs },
    })
    expect(w.findComponent({ name: 'RruleBuilder' }).props('modelValue')).toBe('')
  })

  it('maps disabled to the builder readonly prop', () => {
    const w = mount(RruleWidget, {
      props: { modelValue: '', mode: 'edit' as const, disabled: true, propertyDef: { type: 'rrule' } },
      global: { stubs },
    })
    expect(w.findComponent({ name: 'RruleBuilder' }).props('readonly')).toBe(true)
  })

  it('re-emits builder updates as update:modelValue', async () => {
    const w = mount(RruleWidget, {
      props: { modelValue: '', mode: 'edit' as const, propertyDef: { type: 'rrule' } },
      global: { stubs },
    })
    w.findComponent({ name: 'RruleBuilder' }).vm.$emit('update:modelValue', 'FREQ=WEEKLY')
    await w.vm.$nextTick()
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['FREQ=WEEKLY'])
  })
})

// Display-mode coverage (TKT-UD7YR). Each wrapper widget owns its own
// display rendering -- MultiSelectWidget loops over its array
// internally (RR-UD1G); RruleWidget reuses formatValue.

describe('MultiSelectWidget (display)', () => {
  it('renders a Badge per array value (widget owns its multiplicity)', () => {
    const w = mount(MultiSelectWidget, {
      props: {
        modelValue: ['a', 'b'],
        mode: 'display' as const,
        propertyDef: { type: 'enum', values: ['a', 'b', 'c'] },
        propertyName: 'tags',
      },
    })
    const badges = w.findAllComponents({ name: 'Badge' })
    expect(badges).toHaveLength(2)
    expect(badges[0].props('value')).toBe('a')
    expect(badges[0].props('property')).toBe('tags')
    expect(badges[1].props('value')).toBe('b')
  })

  it('renders nothing for an empty array', () => {
    const w = mount(MultiSelectWidget, {
      props: { modelValue: [], mode: 'display' as const, propertyDef: { type: 'enum' } },
    })
    expect(w.findAllComponents({ name: 'Badge' })).toHaveLength(0)
  })

  it('coerces a scalar into a single Badge', () => {
    const w = mount(MultiSelectWidget, {
      props: { modelValue: 'solo', mode: 'display' as const, propertyDef: { type: 'enum' } },
    })
    const badges = w.findAllComponents({ name: 'Badge' })
    expect(badges).toHaveLength(1)
    expect(badges[0].props('value')).toBe('solo')
  })
})

describe('RruleWidget (display)', () => {
  it('renders a human-readable summary via formatValue', () => {
    const w = mount(RruleWidget, {
      props: { modelValue: 'FREQ=DAILY', mode: 'display' as const, propertyDef: { type: 'rrule' } },
      global: { stubs: { RruleBuilder: true } },
    })
    expect(w.findComponent({ name: 'RruleBuilder' }).exists()).toBe(false)
    // formatValue('FREQ=DAILY', 'rrule') uses RRule.toText() which
    // produces something like "every day"; assert non-empty + lower-cased
    // English summary rather than coupling to the exact phrasing.
    const text = w.find('span.display-value').text()
    expect(text.length).toBeGreaterThan(0)
    expect(text.toLowerCase()).not.toContain('freq=')
  })

  it('renders empty for null', () => {
    const w = mount(RruleWidget, {
      props: { modelValue: null, mode: 'display' as const, propertyDef: { type: 'rrule' } },
      global: { stubs: { RruleBuilder: true } },
    })
    expect(w.find('span.display-value').text()).toBe('')
  })

  it('falls back to the raw string for un-parseable rrule', () => {
    const w = mount(RruleWidget, {
      props: { modelValue: 'not-an-rrule', mode: 'display' as const, propertyDef: { type: 'rrule' } },
      global: { stubs: { RruleBuilder: true } },
    })
    // formatValue catches RRule.fromString errors and returns the raw
    // input; the widget displays whatever formatValue returns.
    expect(w.find('span.display-value').text()).toBe('not-an-rrule')
  })
})
