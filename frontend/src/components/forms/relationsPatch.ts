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
//    from RelationPicker's `update:types` emit.
//
// Both emit deltas (`add` / `remove`), never the full set; see
// ConfirmedEdges.

import type {
  ModernRelationsField,
  RelationsDelta,
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

// The address of a relation row as the server serves it. An incoming
// content-scoped edge belongs to one face of its source, so `id@face` names
// it; every other row is its id. The widgets key their rows by this address,
// so a RelationEntry's `id` in a RelationCardState is already the address.
export function entryAddress(e: { id: string; face?: string }): string {
  return e.face ? `${e.id}@${e.face}` : e.id
}

// A loaded row keyed by its address; `face` stays for display.
export function addressedEntry(e: RelationEntry): RelationEntry {
  return { ...e, id: entryAddress(e) }
}

// The edges the server holds, per body key: address → peer type (undefined
// when only the address is known). Seeded from what the form loaded, then
// advanced by confirmRelations after each saved PATCH.
//
// A form sends deltas, never the full set: it can only see part of an edge
// set (a face the principal cannot read is absent), so a full set would
// delete what it cannot see. A delta is computed against this record rather
// than against the loaded state, so an edge added by one autosave and removed
// before the next is still deleted. Re-sending a delta is harmless: `add` is
// an upsert and removing an absent edge is a no-op.
export type ConfirmedEdges = Map<string, Map<string, string | undefined>>

// The confirmed edges of bodyKey, seeding them from seed() on first use.
function confirmedFor(
  confirmed: ConfirmedEdges,
  bodyKey: string,
  seed: () => Map<string, string | undefined>
): Map<string, string | undefined> {
  let base = confirmed.get(bodyKey)
  if (!base) {
    base = seed()
    confirmed.set(bodyKey, base)
  }
  return base
}

// Advance confirmed by a relations body the server accepted.
export function confirmRelations(confirmed: ConfirmedEdges, body: ModernRelationsField): void {
  for (const [bodyKey, upd] of Object.entries(body)) {
    const base = confirmed.get(bodyKey) ?? new Map<string, string | undefined>()
    confirmed.set(bodyKey, base)
    if ('data' in upd) {
      base.clear()
      for (const ri of upd.data) base.set(ri.id, ri.type)
      continue
    }
    for (const ri of upd.remove ?? []) base.delete(ri.id)
    for (const ri of upd.add ?? []) base.set(ri.id, ri.type)
  }
}

// The edges a card state was loaded with: the current entries, less what
// was added, plus what was removed.
function loadedEdges(state: RelationCardState): Map<string, string | undefined> {
  const out = new Map<string, string | undefined>(state.entries.map((e) => [e.id, e.type]))
  for (const a of state.added) out.delete(a.targetId)
  for (const r of state.removed) if (!out.has(r)) out.set(r, undefined)
  return out
}

// One delta wrapper, or null when it changes nothing.
function deltaOf(
  desired: ResourceIdentifier[],
  base: Map<string, string | undefined>,
  updated: Set<string>
): RelationsDelta | null {
  const want = new Set(desired.map((ri) => ri.id))
  const add = desired.filter((ri) => !base.has(ri.id) || updated.has(ri.id))
  const remove = [...base.entries()]
    .filter(([id]) => !want.has(id))
    .map(([id, type]) => (type ? { type, id } : { id }))
  if (add.length === 0 && remove.length === 0) return null
  const out: RelationsDelta = {}
  if (add.length > 0) out.add = add
  if (remove.length > 0) out.remove = remove
  return out
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
//   - Each key becomes a delta (`add` / `remove`) against `confirmed`
//     (see ConfirmedEdges), and is omitted when it changes nothing. Entry
//     ids are addresses (addressedEntry).
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
  confirmed: ConfirmedEdges = new Map()
): ModernRelationsField {
  const out: ModernRelationsField = {}
  for (const [key, state] of pending.entries()) {
    const isOutgoing = key.endsWith(OUTGOING_SUFFIX)
    const isIncoming = key.endsWith(INCOMING_SUFFIX)
    if (!isOutgoing && !isIncoming) continue
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
            `no inverse declared in metamodel. DynamicForm should have pre-flighted this.`
        )
      }
      bodyKey = inverse
    }
    const desired: ResourceIdentifier[] = state.entries.map((e) => {
      if (!e.type) {
        throw new Error(
          `RelationEntry ${e.id} missing 'type'. ` +
            `Backend or RelationCards drift — refusing to emit a malformed PATCH.`
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
    const base = confirmedFor(confirmed, bodyKey, () => loadedEdges(state))
    const delta = deltaOf(desired, base, new Set(state.updated.map((u) => u.targetId)))
    if (delta) out[bodyKey] = delta
  }
  return out
}

// Reshape a legacy IDs-only relations record into the modern shape
// using a per-relation `pickerTypes` map. Each relation becomes a delta
// against `confirmed`, seeded from `loaded` (the ids the form loaded), and is
// omitted when it changes nothing.
//
// Returns null if an added ID has no resolved type. Caller's choice on
// fallback (DynamicForm falls back to a legacy body + warning toast).
export function reshapeLegacyToModern(
  legacy: Record<string, string[]>,
  pickerTypes: Record<string, Map<string, string>>,
  confirmed: ConfirmedEdges = new Map(),
  loaded: Record<string, string[]> = {}
): ModernRelationsField | null {
  const out: ModernRelationsField = {}
  for (const [relation, ids] of Object.entries(legacy)) {
    const types = pickerTypes[relation]
    const base = confirmedFor(
      confirmed,
      relation,
      () => new Map((loaded[relation] ?? []).map((id) => [id, types?.get(id)]))
    )
    const desired: ResourceIdentifier[] = []
    for (const id of ids) {
      if (base.has(id)) {
        desired.push({ type: base.get(id) ?? '', id })
        continue
      }
      const type = types?.get(id)
      if (!type) return null
      desired.push({ type, id })
    }
    const delta = deltaOf(desired, base, new Set())
    if (delta) out[relation] = delta
  }
  return out
}
