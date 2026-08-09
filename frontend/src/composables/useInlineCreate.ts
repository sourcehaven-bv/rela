import { computed, inject, provide, type ComputedRef, type Ref } from 'vue'
import { useSchemaStore } from '@/stores'

/**
 * Injection key for the inline-create nesting depth. 0 (the default when
 * nothing has provided it) means "top-level form"; a nested create modal
 * provides depth + 1.
 */
export const INLINE_CREATE_DEPTH = Symbol('inlineCreateDepth')

/**
 * Deepest level at which the inline-create affordance is offered. At depth 0
 * (a top-level form) a relation field may spawn one create modal; the form
 * inside that modal is at depth 1 and offers link-existing only.
 *
 * This cap is STRUCTURAL, not advisory (TKT-OMUD56 / RR-UTURB1). Without it a
 * nested form's RelationPicker would call this same composable, get the same
 * eligible types, and open a modal on top of a modal — and `modalStack.ts` is
 * a Set, not a stack, so it cannot say which modal is topmost. Escape would
 * have no defined recipient and both overlays sit at the same z-index. Rather
 * than coordinate that, we make it unreachable.
 */
const MAX_INLINE_CREATE_DEPTH = 0

/** One offerable inline-create target. */
export interface InlineCreateTarget {
  /** Entity type to create. */
  entityType: string
  /** Human label for the button ("+ New Feature"). */
  label: string
  /** Form id to render in the modal, resolved server-side. */
  formId: string
}

/**
 * Provide the nesting depth for everything rendered below — call this from a
 * component that hosts a nested create form (the inline-create modal).
 */
export function provideInlineCreateDepth(): void {
  const current = inject<number>(INLINE_CREATE_DEPTH, 0)
  provide(INLINE_CREATE_DEPTH, current + 1)
}

/**
 * The inline-create targets offerable for a relation's candidate entity types.
 *
 * A type is offered only when the server put it in `inline_create`, which it
 * does only when BOTH conditions hold: the principal may create that type, and
 * a create form resolves for it. So this composable performs no permission
 * arithmetic — presence in the map IS the affordance (see
 * `SidebarData.inline_create`).
 *
 * Returns an empty list when the current form is itself nested (see
 * MAX_INLINE_CREATE_DEPTH), or before the sidebar payload has landed.
 */
export function useInlineCreate(
  targetTypes: Ref<string[]> | ComputedRef<string[]>,
): ComputedRef<InlineCreateTarget[]> {
  const schemaStore = useSchemaStore()
  const depth = inject<number>(INLINE_CREATE_DEPTH, 0)

  return computed(() => {
    if (depth > MAX_INLINE_CREATE_DEPTH) return []

    const out: InlineCreateTarget[] = []
    for (const entityType of targetTypes.value) {
      const formId = schemaStore.inlineCreateFormFor(entityType)
      if (!formId) continue
      out.push({
        entityType,
        label: schemaStore.getEntityType(entityType)?.label || entityType,
        formId,
      })
    }
    return out
  })
}
