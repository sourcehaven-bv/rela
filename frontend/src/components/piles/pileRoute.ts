import type { RouteLocationRaw } from 'vue-router'

/** The scope query that walks a pile: `?from=pile&pile=<id>`, plus the world. */
export function pileScopeQuery(pileId: string, world?: string): Record<string, string> {
  return world ? { from: 'pile', pile: pileId, world } : { from: 'pile', pile: pileId }
}

/**
 * The entity page of a pile item, stepping through the pile.
 *
 * Addressed by the item's ADDRESS (`POL-1@draft`), not its bare id: the pile
 * holds that face, and a bare id would let the world pick another one.
 */
export function pileItemRoute(
  item: { type: string; address: string },
  pileId: string,
  world?: string
): RouteLocationRaw {
  return { path: `/entity/${item.type}/${item.address}`, query: pileScopeQuery(pileId, world) }
}
