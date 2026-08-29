import { api } from './client'

/** GanttSpan is a nullable date interval; "YYYY-MM-DD" strings. */
export interface GanttSpan {
  start?: string
  end?: string
}

/** GanttBreach flags the rolled span escaping the planned window. */
export interface GanttBreach {
  before?: boolean
  after?: boolean
}

/**
 * GanttNode is one entity in the server-folded containment tree. `planned`
 * (the entity's own window) and `rolled` (its descendants' envelope) arrive
 * separately on purpose — their disagreement IS the breach signal.
 */
export interface GanttNode {
  id: string
  type: string
  title?: string
  color?: string
  planned?: GanttSpan
  rolled?: GanttSpan
  committed?: string
  breach?: GanttBreach
  children?: GanttNode[]
  /** True when children exist that this response does not carry (depth cap
   * or node budget) — the drill signal for a node that looks like a leaf. */
  has_more_children?: boolean
}

export interface GanttResponse {
  roots: GanttNode[]
  /** True when the node cap cut the (ACL-filtered) emission short. */
  truncated?: boolean
}

/** getGantt fetches the folded tree for a configured gantt, optionally
 * re-rooted at one entity (server-side drill for trees past the cap). */
export async function getGantt(id: string, root?: string): Promise<GanttResponse> {
  const params = root ? { root } : undefined
  return api.get<GanttResponse>(`/_gantts/${id}`, params)
}
