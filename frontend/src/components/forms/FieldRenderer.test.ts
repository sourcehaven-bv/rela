// TKT-G7N5 AC8 (partial): FieldRenderer correctly consumes the
// affordance plumbing — readonly + option-verdicts. The hidden-field
// filter at the form level is exercised separately in
// DynamicForm.affordances.test.ts (which uses a focused harness to
// avoid the full DynamicForm mount cost).

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import FieldRenderer from './FieldRenderer.vue'
import type { FormFieldOrRelation, PropertyDef } from '@/types'

function renderField(opts: {
  field: FormFieldOrRelation
  propertyDef?: PropertyDef
  value: unknown
  readonly?: boolean
  optionVerdicts?: Record<string, boolean>
}) {
  return mount(FieldRenderer, {
    props: {
      field: opts.field,
      propertyDef: opts.propertyDef,
      value: opts.value,
      readonly: opts.readonly,
      optionVerdicts: opts.optionVerdicts,
    },
    attachTo: document.body,
  })
}

describe('FieldRenderer affordance plumbing', () => {
  it('readonly text input rendered with disabled attribute', () => {
    // FieldRenderer's standard text input uses :disabled, not
    // :readonly, to disable both editing AND focus — matches existing
    // SPA convention.
    const wrapper = renderField({
      field: { property: 'title', label: 'Title' },
      propertyDef: { type: 'string' },
      value: 'hello',
      readonly: true,
    })
    const input = wrapper.find('input[type="text"]')
    expect(input.exists()).toBe(true)
    expect(input.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('readonly enum select rendered with disabled attribute', () => {
    const wrapper = renderField({
      field: { property: 'kind', label: 'Kind' },
      propertyDef: { type: 'enum', values: ['enhancement', 'refactor'] },
      value: 'enhancement',
      readonly: true,
    })
    const select = wrapper.find('select')
    expect(select.exists()).toBe(true)
    expect(select.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('option-verdicts disable specific options on a writable select', () => {
    const wrapper = renderField({
      field: { property: 'status', label: 'Status' },
      propertyDef: { type: 'enum', values: ['open', 'review', 'done'] },
      value: 'open',
      optionVerdicts: { done: false }, // only the false entry appears
    })
    const select = wrapper.find('select')
    expect(select.exists()).toBe(true)
    // Whole select is NOT disabled (writable).
    expect(select.attributes('disabled')).toBeUndefined()

    const opts = wrapper.findAll('option')
    const byValue = Object.fromEntries(
      opts.map((o) => [o.attributes('value'), o] as const)
    )
    expect(byValue['done']).toBeDefined()
    expect(byValue['done'].attributes('disabled')).toBeDefined()
    // Allowed options are not marked disabled by the verdict.
    expect(byValue['open'].attributes('disabled')).toBeUndefined()
    expect(byValue['review'].attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('no option-verdicts means no options disabled (sparse default)', () => {
    const wrapper = renderField({
      field: { property: 'status', label: 'Status' },
      propertyDef: { type: 'enum', values: ['open', 'done'] },
      value: 'open',
    })
    const opts = wrapper.findAll('option')
    for (const opt of opts) {
      expect(opt.attributes('disabled')).toBeUndefined()
    }
    wrapper.unmount()
  })

  it('option-verdicts and transition rules both apply', () => {
    // Affordance denies 'done'; transition rules also restrict the
    // pickable set. The visible-but-disabled rendering applies in
    // either case (TKT-G7N5 + existing transition path).
    const wrapper = renderField({
      field: {
        property: 'status',
        label: 'Status',
        transitions: { open: ['review'] }, // can't go open→done via transitions
      },
      propertyDef: { type: 'enum', values: ['open', 'review', 'done'] },
      value: 'open',
      optionVerdicts: { done: false },
    })
    const opts = wrapper.findAll('option')
    const byValue = Object.fromEntries(
      opts.map((o) => [o.attributes('value'), o] as const)
    )
    // 'done' is disabled by BOTH the affordance and the transition rule.
    expect(byValue['done'].attributes('disabled')).toBeDefined()
    // 'review' is allowed by both.
    expect(byValue['review'].attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  // TagSelect readonly/option-verdict plumbing is verified by the
  // wire-shape contract in api_v1_test.go (server-side
  // checkEnumOption coverage) plus by the typecheck — happy-dom
  // can't mount SlimSelect's MutationObserver cleanly, so a direct
  // component test isn't worth the harness fight here.
})
