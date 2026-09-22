/**
 * Decides which relation keys a form may put on the wire (BUG-KQSOJ2).
 *
 * `relations.value` is seeded from the entity GET, which returns EVERY relation
 * the entity has — not just the ones this form renders. The submit paths then
 * sent that whole map, which has two consequences, both bad:
 *
 * 1. A relation with no field on the form has no `pickerTypes` entry (only a
 *    rendered RelationPicker fills one), so `reshapeLegacyToModern` returns
 *    null and the ENTIRE relations payload is dropped — including the edit the
 *    user actually made. That is the "Some related entities have unknown types"
 *    abort, and reloading cannot help: the GET returns the same untyped key
 *    every time, so the field is permanently unsavable rather than transiently.
 *
 * 2. Even when types happen to resolve, the form re-sends a full-replace body
 *    for a relation it never showed. The user cannot see or edit those edges,
 *    so the form has no business asserting their value.
 *
 * Ownership is a property of the form CONFIG, not of what is on screen: an
 * affordance-hidden field still belongs to the form, and wizard-step visibility
 * is applied separately by `pruneWizardHiddenRelations`. So callers pass
 * `allFields`, never the affordance-filtered `fields` — same reasoning as
 * `routePrefilledCardRelations`.
 */

/** The subset of a form field this decision needs. */
export interface RelationFieldLike {
  relation?: string
  direction?: 'outgoing' | 'incoming'
}

/**
 * Relation keys this form owns, in the vocabulary `relations.value` uses.
 *
 * Ownership follows the field's DIRECTION, because only an outgoing field
 * keeps its state in `relations.value` at all:
 *
 *   - An OUTGOING field's picker writes ids to `relations.value[relation]` and
 *     types to `pickerTypes[relation]`. That key is owned.
 *   - An INCOMING field's picker loads its own edges and delivers them through
 *     `pendingCardChanges` under `<relation>-incoming`, which
 *     `buildRelationsPatch` remaps to the inverse body key. It never reads
 *     `relations.value`. So the inverse key the entity GET supplies is NOT
 *     owned: it is a stale, untyped duplicate of what the picker already
 *     delivers, and sending it is what aborted the payload (BUG-KQSOJ2).
 *
 * Hence an incoming field contributes nothing here. Its canonical name is
 * still added — harmlessly, since the GET never keys an incoming edge that way
 * — so that a form declaring BOTH directions of one relation keeps the
 * outgoing half owned regardless of field order.
 */
export function ownedRelationKeys(fields: RelationFieldLike[]): Set<string> {
  const owned = new Set<string>()
  for (const f of fields) {
    if (!f.relation || f.direction === 'incoming') continue
    owned.add(f.relation)
  }
  return owned
}

/**
 * Drop relation keys the form does not own, keeping any explicitly allowed.
 *
 * `alsoKeep` exists for prefilled edges that legitimately have no field on this
 * form: a `link_as: to` create button may pre-link a relation the form does not
 * render, and that path registers its own `pickerTypes` entry so the edge is
 * typed and safe to emit. Dropping it here would turn a working create into the
 * hard error the post-condition check at the submit site raises.
 */
export function filterOwnedRelations(
  relations: Record<string, string[]>,
  owned: Set<string>,
  alsoKeep?: Set<string>
): Record<string, string[]> {
  const out: Record<string, string[]> = {}
  for (const [rel, ids] of Object.entries(relations)) {
    if (owned.has(rel) || alsoKeep?.has(rel)) out[rel] = ids
  }
  return out
}

/**
 * Relation keys that would abort the save for want of a resolved type.
 *
 * Purely for the error message. `reshapeLegacyToModern` answers the same
 * question by returning null, but it cannot say WHICH key was at fault, and
 * "reload the form and try again" is advice that cannot work when the cause is
 * a key the form never renders. Naming the relation puts the operator on the
 * config that needs fixing.
 */
export function untypedRelationKeys(
  relations: Record<string, string[]>,
  pickerTypes: Record<string, Map<string, string>>
): string[] {
  const bad: string[] = []
  for (const [rel, ids] of Object.entries(relations)) {
    const types = pickerTypes[rel]
    if (ids.some((id) => !types?.get(id))) bad.push(rel)
  }
  return bad
}
