import type { RowMove } from 'rela-components/components/table/types'
import type { RelationPosition, RelationType, SidebarPage } from '@/types'

/**
 * Whether a tab of an entity page lists its rows in relation order: its
 * single link runs outgoing from the anchor over a relation orderable on that
 * side. The server makes the same decision for the same tab; the client needs
 * it before the first read, to leave the list's `default_sort` out of it,
 * because a sort sent with the read would replace the relation order.
 */
export function tabIsRelationOrdered(
  page: SidebarPage | undefined,
  tab: string,
  relations: ReadonlyMap<string, RelationType>
): boolean {
  const links = page?.tabs.find((t) => t.id === tab)?.links
  if (links?.length !== 1) return false
  const [link] = links
  return link.direction === 'outgoing' && relations.get(link.relation)?.orderable?.outgoing === true
}

/** A row move resolved against the rows on screen. */
export interface PlannedMove<T> {
  /** The rows in their new order, to show before the server answers. */
  rows: T[]
  /** What to send the server. */
  position: RelationPosition
}

/**
 * Resolves a move the table reported into the rows' new order and the
 * position to send, or null when the move changes nothing.
 *
 * A step moves past the neighbour on screen, named by id so the server
 * places the row exactly where the reader saw it go. At the first or last row
 * on screen there is no neighbour to name, so the step itself is sent: the
 * server knows the rows on the next page, and the refetch shows the result.
 */
export function planRowMove<T extends { id: string }>(rows: T[], move: RowMove): PlannedMove<T> | null {
  const from = rows.findIndex((r) => r.id === move.itemId)
  if (from < 0) return null
  const moved = rows[from]
  const rest = rows.filter((_, i) => i !== from)

  if ('step' in move) {
    const neighbour = rows[from + move.step]
    if (!neighbour) return { rows, position: { step: move.step } }
    const at = from + move.step
    return {
      rows: [...rest.slice(0, at), moved, ...rest.slice(at)],
      position: move.step < 0 ? { before: neighbour.id } : { after: neighbour.id },
    }
  }

  const target = rest.findIndex((r) => r.id === move.targetId)
  if (target < 0) return null
  const at = move.placement === 'before' ? target : target + 1
  if (at === from) return null
  return {
    rows: [...rest.slice(0, at), moved, ...rest.slice(at)],
    position: move.placement === 'before' ? { before: move.targetId } : { after: move.targetId },
  }
}
