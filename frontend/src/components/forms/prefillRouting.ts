import { INCOMING_SUFFIX, OUTGOING_SUFFIX } from './relationsPatch'

/**
 * Decides where each prefilled relation key must be delivered (TKT-Z8K2FS).
 *
 * `relations.value` is NOT the create payload for a `widget: cards` relation:
 * the submit path excludes those keys and reads `pendingCardChanges` instead.
 * A prefill that writes only the former loses every card-managed edge — the
 * entity is created, the edges are not, and nothing errors. That is RR-7Z3SFC's
 * failure mode, fixed once for a single pre-linked peer and reappearing here at
 * N peers across N relations.
 *
 * Pure on purpose: the routing decision is where the subtlety lives (inverse
 * keys, symmetric self-inverse relations), while the mutation it drives is
 * mechanical. Keeping them apart makes the decision directly testable instead
 * of only observable through a full form submit.
 */
export interface PrefillPeer {
  id: string
  type: string
}

export interface CardRoute {
  /** Key into `pendingCardChanges`, i.e. `<relation><direction-suffix>`. */
  mapKey: string
  /** The wire key this came in under, to be dropped from `relations.value`. */
  sourceKey: string
  peers: PrefillPeer[]
}

/**
 * Splits prefilled relations into card-delivered routes and the keys that stay
 * on the ordinary picker path.
 *
 * `inverseByRelation` maps a canonical relation name to its declared inverse.
 * `cardRelations` holds the canonical names whose form field is card-managed.
 */
export function planPrefillRouting(
  prefilled: Record<string, PrefillPeer[]>,
  cardRelations: Set<string>,
  inverseByRelation: Map<string, string>
): { cardRoutes: CardRoute[] } {
  const canonicalByInverse = new Map<string, string>()
  for (const [canonical, inverse] of inverseByRelation) {
    canonicalByInverse.set(inverse, canonical)
  }

  const cardRoutes: CardRoute[] = []
  for (const [key, peers] of Object.entries(prefilled)) {
    const mapped = canonicalByInverse.get(key)
    // A SYMMETRIC self-inverse relation (`symmetric: true` with
    // `inverse.id == relType`) is legal — the one permitted name overlap — and
    // maps to ITSELF, so presence in the lookup cannot be the direction test.
    // Such a key is its own inverse and is written OUTGOING by convention,
    // matching resolveDirection server-side.
    const isInverseKey = mapped !== undefined && mapped !== key
    const relation = isInverseKey ? mapped : key
    if (!cardRelations.has(relation)) continue

    const suffix = isInverseKey ? INCOMING_SUFFIX : OUTGOING_SUFFIX
    cardRoutes.push({ mapKey: `${relation}${suffix}`, sourceKey: key, peers })
  }
  return { cardRoutes }
}
