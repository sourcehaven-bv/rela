// Scoped relations (TKT-5GK80Q9): a relation with `within:` may only point to
// targets owned by the entity its scope relation points to. The server
// enforces this on every write; these helpers let a form offer only the
// targets it would accept and react when the scope value changes.
import type { RelationType } from '@/types'

type RelationTypes = Record<string, RelationType>

// One request that lists the targets an owner holds: the edges of
// `owningRelation` leaving the scope entity, read as `ownerType`, kept when
// the target is of `targetType`.
export interface OwnerQuery {
  ownerType: string
  owningRelation: string
  targetType: string
}

// The queries that list the targets relation `name` may point to once its
// scope relation points somewhere. Empty for a relation that is not scoped.
export function ownerQueries(name: string, rels: RelationTypes): OwnerQuery[] {
  const rel = rels[name]
  const scope = rel?.within ? rels[rel.within] : undefined
  if (!rel || !scope) return []
  const out: OwnerQuery[] = []
  for (const targetType of rel.to) {
    const owningRelation = rel.scope_owners?.[targetType]
    const owning = owningRelation ? rels[owningRelation] : undefined
    if (!owningRelation || !owning) continue
    for (const ownerType of scope.to) {
      if (owning.from.includes(ownerType)) out.push({ ownerType, owningRelation, targetType })
    }
  }
  return out
}

// The relations scoped within relation `scopeName`, with what a change of the
// scope does to each.
export function dependentsOf(
  scopeName: string,
  rels: RelationTypes
): Array<{ relation: string; onChange: 'clear' | 'reject' }> {
  const out: Array<{ relation: string; onChange: 'clear' | 'reject' }> = []
  for (const [relation, def] of Object.entries(rels)) {
    if (def.within === scopeName && def.on_change) out.push({ relation, onChange: def.on_change })
  }
  return out.sort((a, b) => a.relation.localeCompare(b.relation))
}

// Whether two id lists hold the same ids, in any order.
export function sameIds(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false
  const set = new Set(a)
  return b.every((id) => set.has(id))
}
