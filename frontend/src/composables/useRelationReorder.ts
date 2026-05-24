// useRelationReorder centralises the math used by drag-to-reorder on
// orderable relation list sections. The composable does NOT issue any
// network calls — the caller integrates the returned value into its
// existing relation-update flow (RelationCardState.updated → PATCH).
//
// The midpoint scheme matches the backend's MidpointOrder in
// internal/entitymanager/order.go. Keep the constants in sync.

export const ORDER_COLLAPSE_THRESHOLD = 1e-9

export interface ReorderNeighbors {
  /**
   * The numeric order value of the entry immediately above the moved
   * entry after the drop, or undefined if the moved entry now sits at
   * the top of the list.
   */
  prevOrder: number | undefined
  /**
   * The numeric order value of the entry immediately below the moved
   * entry after the drop, or undefined if the moved entry now sits at
   * the bottom of the list.
   */
  nextOrder: number | undefined
}

/**
 * computeNewOrder returns the order value the moved entry should be
 * assigned given its new neighbours. Returns undefined when no safe
 * midpoint exists — the caller should request a backend renumber by
 * issuing a PATCH that sets the order to the midpoint anyway (the
 * backend's collapse detection will trigger a renumber pass) or, if
 * preferable, skip the optimistic local update entirely.
 */
export function computeNewOrder(n: ReorderNeighbors): number | undefined {
  const hasPrev = n.prevOrder !== undefined && Number.isFinite(n.prevOrder)
  const hasNext = n.nextOrder !== undefined && Number.isFinite(n.nextOrder)

  if (!hasPrev && !hasNext) {
    return 1.0
  }
  if (!hasPrev && hasNext) {
    return (n.nextOrder as number) - 1.0
  }
  if (hasPrev && !hasNext) {
    return (n.prevOrder as number) + 1.0
  }
  const lo = n.prevOrder as number
  const hi = n.nextOrder as number
  if (hi - lo < ORDER_COLLAPSE_THRESHOLD) {
    return undefined
  }
  return lo + (hi - lo) / 2
}

/**
 * extractFiniteNumber reads a meta property as a finite number. Returns
 * undefined when the value is missing, non-numeric, or non-finite. Used
 * by drag-end handlers to read neighbour order values from the existing
 * entries list.
 */
export function extractFiniteNumber(v: unknown): number | undefined {
  if (typeof v === 'number' && Number.isFinite(v)) {
    return v
  }
  return undefined
}
