/**
 * A list toggle column (TKT-FJG779Q): a checkbox for a boolean or enum
 * property. The server decides everything the cell shows (`_toggles`):
 * whether there is a checkbox, whether it is checked, and what a click
 * writes. This module only reads that back.
 */
import type { ListColumn } from '@/types/config'
import type { Entity } from '@/types'

export interface ToggleCellState {
  /** No checkbox: the row has no `_toggles` entry for the property. */
  hidden: boolean
  checked: boolean
  /** True when the server offered no write. */
  disabled: boolean
  /** The value a click writes; undefined when disabled. */
  target?: string | boolean
}

/** Whether a list column is a toggle column. */
export function isToggleColumn(column: ListColumn | undefined): boolean {
  return column?.widget === 'toggle' && !!column.property
}

/** What a toggle cell shows for one row. */
export function toggleCellState(entity: Entity, column: ListColumn): ToggleCellState {
  const cell = entity._toggles?.[column.property ?? '']
  if (!cell) return { hidden: true, checked: false, disabled: true }
  return { hidden: false, checked: cell.checked, disabled: cell.to === undefined, target: cell.to }
}
