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
      props: { modelValue: ['a', 1], propertyDef: { type: 'enum', values: ['a', '1', 'b'] } },
      global: { stubs },
    })
    const tag = w.findComponent({ name: 'TagSelect' })
    expect(tag.props('modelValue')).toEqual(['a', '1'])
    expect(tag.props('options')).toEqual(['a', '1', 'b'])
  })

  it('coerces a scalar model value to a single-item array', () => {
    const w = mount(MultiSelectWidget, {
      props: { modelValue: 'solo', propertyDef: { type: 'enum' } },
      global: { stubs },
    })
    expect(w.findComponent({ name: 'TagSelect' }).props('modelValue')).toEqual(['solo'])
  })

  it('treats empty model value as an empty array', () => {
    const w = mount(MultiSelectWidget, {
      props: { modelValue: null, propertyDef: { type: 'enum' } },
      global: { stubs },
    })
    expect(w.findComponent({ name: 'TagSelect' }).props('modelValue')).toEqual([])
  })

  it('re-emits TagSelect updates as update:modelValue', async () => {
    const w = mount(MultiSelectWidget, {
      props: { modelValue: [], propertyDef: { type: 'enum' } },
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
      props: { modelValue: 'FREQ=DAILY', help: 'every day', propertyDef: { type: 'rrule' } },
      global: { stubs },
    })
    const builder = w.findComponent({ name: 'RruleBuilder' })
    expect(builder.props('modelValue')).toBe('FREQ=DAILY')
    expect(builder.props('help')).toBe('every day')
  })

  it('renders empty for a null model value', () => {
    const w = mount(RruleWidget, {
      props: { modelValue: null, propertyDef: { type: 'rrule' } },
      global: { stubs },
    })
    expect(w.findComponent({ name: 'RruleBuilder' }).props('modelValue')).toBe('')
  })

  it('maps disabled to the builder readonly prop', () => {
    const w = mount(RruleWidget, {
      props: { modelValue: '', disabled: true, propertyDef: { type: 'rrule' } },
      global: { stubs },
    })
    expect(w.findComponent({ name: 'RruleBuilder' }).props('readonly')).toBe(true)
  })

  it('re-emits builder updates as update:modelValue', async () => {
    const w = mount(RruleWidget, {
      props: { modelValue: '', propertyDef: { type: 'rrule' } },
      global: { stubs },
    })
    w.findComponent({ name: 'RruleBuilder' }).vm.$emit('update:modelValue', 'FREQ=WEEKLY')
    await w.vm.$nextTick()
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['FREQ=WEEKLY'])
  })
})
