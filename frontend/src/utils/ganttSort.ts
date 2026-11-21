/**
 * Row order of a gantt: the siblings of every level, by title or by date.
 *
 * Client-side on purpose. The server returns the tree folded and gated, and
 * the order of siblings is presentation, not data: sorting what was returned
 * reveals nothing the response did not already carry.
 *
 * A date is the node's rolled span when it has one, else its own planned
 * window: the span the bar shows. A node with no date sorts after every dated
 * node, in either direction, so a descending sort does not lead with the rows
 * that have nothing to show.
 */
import type { GanttNode } from '@/api'
import type { SortSpec } from '@/types'

export const GANTT_SORT_KEYS = ['title', 'start', 'end'] as const

function dateOf(node: GanttNode, key: 'start' | 'end'): string | undefined {
  return node.rolled?.[key] || node.planned?.[key] || undefined
}

function compare(a: GanttNode, b: GanttNode, spec: SortSpec): number {
  const sign = spec.direction === 'desc' ? -1 : 1
  if (spec.property === 'title') {
    return sign * (a.title || a.id).localeCompare(b.title || b.id, undefined, { sensitivity: 'base' })
  }
  if (spec.property !== 'start' && spec.property !== 'end') return 0
  const left = dateOf(a, spec.property)
  const right = dateOf(b, spec.property)
  if (!left && !right) return 0
  if (!left) return 1
  if (!right) return -1
  // ISO dates compare as text.
  return left < right ? -sign : left > right ? sign : 0
}

/** The nodes with every level's siblings ordered by `specs`; the input is not changed. */
export function sortGanttNodes(nodes: GanttNode[], specs: SortSpec[]): GanttNode[] {
  if (!specs.length) return nodes
  const ordered = [...nodes].sort((a, b) => {
    for (const spec of specs) {
      const c = compare(a, b, spec)
      if (c !== 0) return c
    }
    return 0
  })
  return ordered.map((node) =>
    node.children?.length ? { ...node, children: sortGanttNodes(node.children, specs) } : node,
  )
}
