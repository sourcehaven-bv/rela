// Patch-builder helpers for the unified PATCH-with-relations endpoint
// landed in TKT-6WLSW. Consumed by DynamicForm.handleSubmit.
//
// Two responsibilities:
//
// 1. buildRelationsPatch turns the per-relation pendingCardChanges Map
//    (kept by RelationCards via the cards-changed event) into the
//    JSON:API §9 modern relations field carried on the PATCH body.
//
// 2. reshapeLegacyToModern converts a legacy IDs-only relations record
//    into the modern shape using a per-relation Map<id, type> sourced
//    from RelationPicker's `update:types` emit. Used when card edits
//    force the whole body to modern (the wire format forbids mixing).

import type {
  ModernRelationsField,
  ResourceIdentifier,
  RelationEntry,
} from '@/types'

// Suffix keys used by DynamicForm's pendingCardChanges Map to
// distinguish outgoing vs incoming card-managed widgets. The builder
// must understand these to skip incoming entries (they take the
// per-edge path), and so DynamicForm/RelationCards don't sprinkle
// string literals.
export const OUTGOING_SUFFIX = '-outgoing'
export const INCOMING_SUFFIX = '-incoming'

// Structural shape of RelationCards' cards-changed payload. Defined
// inline (rather than re-imported from RelationCards.vue) to keep this
// module free of Vue-SFC imports — Vitest's vue-tsc can chew on the
// .vue, but plain .ts helpers should not depend on SFC types.
export interface RelationCardState {
  entries: RelationEntry[]
  added: Array<{ targetId: string; meta?: Record<string, unknown> }>
  removed: string[]
  updated: Array<{ targetId: string; meta: Record<string, unknown> }>
}

// Build the modern relations field for the unified PATCH from the
// pending card-changes Map. Contract:
//
//   - Input keys are `<relation>${OUTGOING_SUFFIX}` or
//     `<relation>${INCOMING_SUFFIX}`. Incoming keys are skipped here;
//     they take the per-edge path.
//   - Emit a relation entry ONLY when the user actually touched it
//     during this form session (added/removed/updated non-empty).
//     A pristine card in the Map produces no key — preventing
//     `data: []` wipes on autosave with a stale Map (TKT-ZEKO4 Q6).
//   - `state.entries` is the post-edit desired set; the builder maps
//     it to resource identifiers directly.
//   - Every entry MUST carry `type` (backend Step 0 + RelationCards
//     populate). The builder throws on missing `type` to surface a
//     drift bug loudly instead of emitting a malformed body.
export function buildRelationsPatch(
  pending: Map<string, RelationCardState>,
): ModernRelationsField {
  const out: ModernRelationsField = {}
  for (const [key, state] of pending.entries()) {
    if (key.endsWith(INCOMING_SUFFIX)) continue
    if (!key.endsWith(OUTGOING_SUFFIX)) continue
    if (
      state.added.length === 0 &&
      state.removed.length === 0 &&
      state.updated.length === 0
    ) {
      continue
    }
    const relation = key.slice(0, -OUTGOING_SUFFIX.length)
    const data: ResourceIdentifier[] = state.entries.map((e) => {
      if (!e.type) {
        throw new Error(
          `RelationEntry ${e.id} missing 'type'. ` +
            `Backend or RelationCards drift — refusing to emit a malformed PATCH.`,
        )
      }
      const ri: ResourceIdentifier = { type: e.type, id: e.id }
      if (e.meta && Object.keys(e.meta).length > 0) {
        ri.meta = { ...e.meta }
      }
      if (e.content !== undefined) {
        ri.content = e.content
      }
      return ri
    })
    out[relation] = { data }
  }
  return out
}

// Reshape a legacy IDs-only relations record into the modern shape
// using a per-relation `pickerTypes` map.
//
// Returns null if any ID has no resolved type. Caller's choice on
// fallback (DynamicForm falls back to a legacy body + warning toast).
//
// Empty `[]` for a relation type maps to `{data: []}` which clears
// all edges of that type on the server — exactly mirrors legacy
// semantics, so this is correct.
export function reshapeLegacyToModern(
  legacy: Record<string, string[]>,
  pickerTypes: Record<string, Map<string, string>>,
): ModernRelationsField | null {
  const out: ModernRelationsField = {}
  for (const [relation, ids] of Object.entries(legacy)) {
    const types = pickerTypes[relation]
    const data: ResourceIdentifier[] = []
    for (const id of ids) {
      const type = types?.get(id)
      if (!type) return null
      data.push({ type, id })
    }
    out[relation] = { data }
  }
  return out
}
