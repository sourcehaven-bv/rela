import type { SectionTone } from 'rela-components/types'
import type { KanbanCellColor } from '@/types'
import { styleTokens } from '@/utils/styleColors'

/**
 * The tone names a cell colour may be (`SemanticTones` in
 * internal/dataentryconfig/colors.go). Only used to pick a default label: the
 * server has already resolved the colour itself to a style class.
 */
const CELL_TONES = ['success', 'warning', 'error', 'info', 'new'] as const
export type CellTone = (typeof CELL_TONES)[number]

/**
 * How a swimlane cell is painted, from its configured colour (TKT-83S9G0Z).
 * The label is the configured one, else the tone's translated name. Undefined
 * when the cell has no colour, a class the SPA cannot paint, or no label to
 * show: a colour without text would be the only cue, so it is not drawn.
 */
export function cellTone(
  cell: KanbanCellColor | undefined,
  toneLabels: Record<CellTone, string>
): SectionTone | undefined {
  if (!cell) return undefined
  const tokens = styleTokens(cell.class)
  if (!tokens) return undefined
  const label =
    cell.label?.trim() ||
    ((CELL_TONES as readonly string[]).includes(cell.color)
      ? toneLabels[cell.color as CellTone]
      : '')
  if (!label) return undefined
  return { ...tokens, label }
}
