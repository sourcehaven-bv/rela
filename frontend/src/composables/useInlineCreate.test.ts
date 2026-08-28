import { describe, it, expect, beforeEach } from 'vitest'
import { defineComponent, h, ref, type Component } from 'vue'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useSchemaStore } from '@/stores/schema'
import {
  useInlineCreate,
  provideInlineCreateDepth,
  type InlineCreateTarget,
} from './useInlineCreate'

/**
 * The offer rule lives on the server (a type is listed in `inline_create` only
 * when the principal may create it AND a form resolves). What the composable
 * owns is the *depth cap* — the structural guarantee that a nested create form
 * cannot itself offer inline create, which is what makes modal-in-modal
 * unreachable (TKT-OMUD56 / RR-UTURB1).
 */

/** Mounts useInlineCreate at `depth` levels of nesting and returns its result. */
function targetsAtDepth(types: string[], depth: number): InlineCreateTarget[] {
  let captured: InlineCreateTarget[] = []

  const Leaf = defineComponent({
    setup() {
      const targets = useInlineCreate(ref(types))
      return () => {
        captured = targets.value
        return h('div')
      }
    },
  })

  // Each Nester is one inline-create modal in the chain. Typed explicitly
  // because it references itself for the recursive case.
  const Nester: Component = defineComponent({
    props: { remaining: { type: Number, required: true } },
    setup(props) {
      provideInlineCreateDepth()
      return () =>
        props.remaining > 1 ? h(Nester, { remaining: props.remaining - 1 }) : h(Leaf)
    },
  })

  mount(depth === 0 ? Leaf : Nester, depth === 0 ? {} : { props: { remaining: depth } })
  return captured
}

describe('useInlineCreate', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const schemaStore = useSchemaStore()
    schemaStore.setInlineCreate({ ticket: 'create_ticket', feature: 'create_feature' })
    schemaStore.entityTypes.set('ticket', { name: 'ticket', label: 'Ticket' } as never)
    schemaStore.entityTypes.set('feature', { name: 'feature', label: 'Feature' } as never)
  })

  it('offers a target per eligible type at top level', () => {
    const targets = targetsAtDepth(['ticket', 'feature'], 0)

    expect(targets).toEqual([
      { entityType: 'ticket', label: 'Ticket', formId: 'create_ticket' },
      { entityType: 'feature', label: 'Feature', formId: 'create_feature' },
    ])
  })

  it('omits a type the server did not list', () => {
    // Absent from `inline_create` means the principal cannot create it, or no
    // form resolves — either way it is not offerable.
    const targets = targetsAtDepth(['ticket', 'concept'], 0)

    expect(targets.map((t) => t.entityType)).toEqual(['ticket'])
  })

  it('offers nothing inside a nested create form (the depth cap)', () => {
    // AC10. Without this a nested form's RelationPicker would open a second
    // modal on top of the first, and modalStack is a Set — it cannot say which
    // dialog is topmost, so Escape would have no defined recipient.
    expect(targetsAtDepth(['ticket', 'feature'], 1)).toEqual([])
  })

  it('stays capped at deeper nesting', () => {
    expect(targetsAtDepth(['ticket'], 2)).toEqual([])
  })

  it('offers nothing before the sidebar payload lands', () => {
    // `inline_create` rides on the sidebar fetch, so it is empty on first
    // paint. The affordance must simply not render rather than flicker in.
    useSchemaStore().setInlineCreate({})

    expect(targetsAtDepth(['ticket'], 0)).toEqual([])
  })

  it('falls back to the type name when the metamodel has no label', () => {
    const schemaStore = useSchemaStore()
    schemaStore.setInlineCreate({ risk: 'create_risk' })

    expect(targetsAtDepth(['risk'], 0)).toEqual([
      { entityType: 'risk', label: 'risk', formId: 'create_risk' },
    ])
  })
})
