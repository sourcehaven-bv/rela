import type { Entity, RelationEntry } from '@/types'

// Type resolution for relation targets the candidate page does not contain
// (BUG-LSCDJK).
//
// A RelationPicker loads `candidates` as a CHOICE list — the first
// `per_page: 100` entities per target type — so it answers "what could I
// pick", not "what did I pick". Past 100 entities of a target type an existing
// link falls outside that page and has no resolvable type, which makes
// `reshapeLegacyToModern` return null and drops the whole relations payload at
// save time.
//
// The edges themselves carry the peer's type (`RelationEntry.type`), which is
// why the `cards` widget was never affected. These helpers turn those edges
// into the entity records the picker needs.

// Ids in `value` that no known entity covers — the set worth a lookup.
// Returns an empty array when everything is already known, which is the
// common case and lets the caller skip the request entirely.
export function missingIds(value: string[], known: Map<string, Entity>): string[] {
  return value.filter((id) => !known.has(id))
}

// Fold freshly-read edges into the already-resolved set.
//
// Merges rather than replaces: a later call resolves only what is missing at
// that moment, so overwriting would discard links an earlier pass resolved.
// Only ids in `wanted` are taken, and only with a non-empty `type` — an edge
// without one cannot produce a valid resource identifier.
//
// `_title` is set to the id explicitly: `entityDisplay` documents `_title` as
// total for an API-sourced entity, and these records are synthesized rather
// than fetched, so they satisfy that contract instead of relying on a
// downstream fallback. The relations endpoint carries no title, so the id is
// the honest value — the picker shows a bare id rather than nothing at all.
export function mergeResolvedLinks(
  existing: Entity[],
  edges: RelationEntry[],
  wanted: Set<string>
): Entity[] {
  const merged = new Map(existing.map((e) => [e.id, e]))
  for (const edge of edges) {
    if (!wanted.has(edge.id) || !edge.type) continue
    merged.set(edge.id, { id: edge.id, type: edge.type, properties: {}, _title: edge.id })
  }
  return [...merged.values()]
}

// Index every entity the picker can name by id: the candidate page plus the
// out-of-page links resolved from the edges.
//
// The two cannot disagree at resolve time — resolved links are only ever
// filled from ids absent here — so precedence matters just after a newly
// created entity is pushed into `candidates`, or after a candidate reload.
// There the list read is the fresher of the two and wins.
export function indexKnownEntities(resolved: Entity[], candidates: Entity[]): Map<string, Entity> {
  const out = new Map<string, Entity>()
  for (const e of resolved) out.set(e.id, e)
  for (const c of candidates) out.set(c.id, c)
  return out
}

// The entities behind `value`, in `value` order so a chip keeps its place once
// its entity resolves, and deduped: mapping over the value list can repeat an
// entity where filtering the candidate list could not. The dropdown hides
// already-selected ids, so a repeat is not reachable through the UI, but
// duplicate stored data would otherwise render two chips whose x buttons
// remove one link. Ids that resolve to nothing are dropped.
export function resolveSelected(value: string[], known: Map<string, Entity>): Entity[] {
  const seen = new Set<string>()
  const out: Entity[] = []
  for (const id of value) {
    if (seen.has(id)) continue
    seen.add(id)
    const entity = known.get(id)
    if (entity) out.push(entity)
  }
  return out
}
