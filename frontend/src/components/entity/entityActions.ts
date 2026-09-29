/**
 * What can be done to one entity, as one list: the commands, Edit, the other
 * faces and copies, History, the export formats, Duplicate and Delete.
 *
 * EntityDetail builds the list once, from the same checks its header buttons
 * use. The phone overflow menu and the detail panel's "⋯" menu both draw it,
 * so an action added to the list cannot be reachable in one place and missing
 * in another. That had happened three times while each place kept its own
 * copy.
 */
import type { IconName } from 'rela-components/components/common/icons'

/** Menu sections, in the order they are drawn. */
export const ENTITY_ACTION_GROUPS = [
  'command',
  'edit',
  'view',
  'export',
  'manage',
  'danger',
] as const

export type EntityActionGroup = (typeof ENTITY_ACTION_GROUPS)[number]

export interface EntityAction {
  /** Stable within one list, for keys and tests. */
  id: string
  label: string
  group: EntityActionGroup
  icon?: IconName
  tone?: 'danger'
  /** The keyboard shortcut, shown beside the label. */
  shortcut?: string
  disabled?: boolean
  /** A download the browser follows. Set this or `run`, not both. */
  href?: string
  run?: () => void
}

/** The actions split into their sections, in drawing order, empty ones left out. */
export function groupEntityActions(actions: EntityAction[]): EntityAction[][] {
  return ENTITY_ACTION_GROUPS.map((group) => actions.filter((a) => a.group === group)).filter(
    (section) => section.length > 0
  )
}
