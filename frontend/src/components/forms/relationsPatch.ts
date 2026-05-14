// Patch-builder helpers for the unified PATCH-with-relations endpoint
// landed in TKT-6WLSW. Consumed by DynamicForm.handleSubmit.
//
// Two responsibilities:
//
// 1. buildRelationsPatch turns the per-relation pendingCardChanges Map
//    (kept by RelationCards via the cards-changed event) into the
//    JSON:API §9 modern relations field carried on the PATCH body.
//    Incoming-direction edits (suffix `-incoming`) are emitted under
//    their relation's inverse name so the backend resolveDirection
//    picks them up as "path entity is target" writes (TKT-GFQK).
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
//     `<relation>${INCOMING_SUFFIX}`.
//   - Outgoing keys map to the canonical relation name as the body key.
//   - Incoming keys map to the relation's inverse name via the
//     `inverseByRelation` lookup. Backend resolveDirection then treats
//     the path entity as the target of the canonical edge.
//   - Emit a relation entry ONLY when the user actually touched it
//     during this form session (added/removed/updated non-empty).
//     A pristine card in the Map produces no key — preventing
//     `data: []` wipes on autosave with a stale Map (TKT-ZEKO4 Q6).
//   - `state.entries` is the post-edit desired set; the builder maps
//     it to resource identifiers directly.
//   - Every entry MUST carry `type`. The builder throws on missing
//     `type` to surface a drift bug loudly instead of emitting a
//     malformed body.
//   - An incoming-suffix key whose canonical relation has no
//     declared inverse also throws. DynamicForm should pre-flight
//     this at form-load time; the throw here is the defensive
//     last-line guard.
export function buildRelationsPatch(
  pending: Map<string, RelationCardState>,
  inverseByRelation: Map<string, string>,
): ModernRelationsField {
  const out: ModernRelationsField = {}
  for (const [key, state] of pending.entries()) {
    const isOutgoing = key.endsWith(OUTGOING_SUFFIX)
    const isIncoming = key.endsWith(INCOMING_SUFFIX)
    if (!isOutgoing && !isIncoming) continue
    if (
      state.added.length === 0 &&
      state.removed.length === 0 &&
      state.updated.length === 0
    ) {
      continue
    }
    const suffixLen = isOutgoing ? OUTGOING_SUFFIX.length : INCOMING_SUFFIX.length
    const canonical = key.slice(0, -suffixLen)
    let bodyKey: string
    if (isOutgoing) {
      bodyKey = canonical
    } else {
      const inverse = inverseByRelation.get(canonical)
      if (!inverse) {
        throw new Error(
          `Cannot emit incoming-direction patch for relation '${canonical}': ` +
            `no inverse declared in metamodel. DynamicForm should have pre-flighted this.`,
        )
      }
      bodyKey = inverse
    }
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
    out[bodyKey] = { data }
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
