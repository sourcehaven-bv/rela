import { DEFAULT_WORLD } from '@/composables/useWorld'
import type { Entity, WorldInfo } from '@/types'
import { entityRef } from '@/utils/entityRef'

/**
 * Candidates for a relation TARGET span every face the reader may read.
 *
 * A relation's head is the entity, not one of its faces (DEC-NPZICR), so a
 * picker that offered only the rows the ambient world serves hid every entity
 * that world excludes: a control could not be related to a policy that exists
 * only as a draft while the page sat in a published world (BUG-FYEEVX).
 *
 * The list API has no "every entity, any readable face" mode, so the picker
 * widens its query over the other declared worlds the reader may select and
 * merges the rows by id. The limit that follows: a face that no readable
 * world serves is still not offered.
 */

/**
 * The declared worlds to query besides the ambient one, sorted so the merge
 * cannot depend on map order. The generated default world is skipped: it
 * serves only the implicit face, which a faced type does not have. A world the reader may not
 * select is skipped too; the server would answer it with an empty list anyway.
 */
export function widenWorlds(worlds: Map<string, WorldInfo>, ambient: string): string[] {
  const out: string[] = []
  for (const [name, info] of worlds) {
    if (name === DEFAULT_WORLD || name === ambient) continue
    if (info.readable === false) continue
    out.push(name)
  }
  return out.sort()
}

/**
 * Merges the ambient world's rows with the rows other worlds serve, one row
 * per entity id. The ambient row wins, so an entity the world serves is
 * labelled by the face the world serves. Otherwise the first world in order
 * that serves the entity supplies the row, which is a face the reader can
 * read. `offWorld` names the ids whose row came from another world.
 */
export function mergeFamilyCandidates(
  ambient: Entity[],
  others: Entity[][]
): { rows: Entity[]; offWorld: Set<string> } {
  const seen = new Set<string>()
  const rows: Entity[] = []
  for (const e of ambient) {
    if (seen.has(e.id)) continue
    seen.add(e.id)
    rows.push(e)
  }
  const offWorld = new Set<string>()
  for (const list of others) {
    for (const e of list) {
      if (seen.has(e.id)) continue
      seen.add(e.id)
      offWorld.add(e.id)
      rows.push(e)
    }
  }
  return { rows, offWorld }
}

/**
 * Candidates for the SOURCE of an incoming content-scoped edge: one row per
 * (entity, face), keyed by address. Such an edge belongs to one face of its
 * source (`POL-1@draft` and `POL-1@published` citing this entity are two
 * edges), so each face the reader can reach is a separate candidate. The
 * first list wins a repeated address.
 */
export function faceCandidates(lists: Entity[][]): Entity[] {
  const seen = new Set<string>()
  const rows: Entity[] = []
  for (const list of lists) {
    for (const e of list) {
      const key = entityRef(e)
      if (seen.has(key)) continue
      seen.add(key)
      rows.push(e)
    }
  }
  return rows
}

/**
 * Whether the dropdown offers a face candidate. A face whose `update` the
 * server denies is left out: a content edge is part of that face's content.
 * This is a hint read off the server's `_actions`; the write re-authorizes.
 */
export function offersFace(e: Entity): boolean {
  return e._actions?.update !== false
}
