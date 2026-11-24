/**
 * A list toggle column (TKT-FJG779Q): a checkbox for a boolean or enum
 * property. The server decides what a click writes and whether the reader
 * may write it (`_toggles`); this module only derives what the cell shows.
 */
import type { ListColumn } from '@/types/config'
import type { Entity } from '@/types'

export interface ToggleCellState {
  /** No checkbox at all: the value is neither on nor off and `other: hidden`. */
  hidden: boolean
  checked: boolean
  /** True when the row carries no write for the property. */
  disabled: boolean
  /** The value a click writes; undefined when disabled. */
  target?: string | boolean
}

/** Whether a list column is a toggle column. */
export function isToggleColumn(column: ListColumn | undefined): boolean {
  return column?.widget === 'toggle' && !!column.property
}

/** What a toggle cell shows for one row. An unset value reads as off. */
export function toggleCellState(entity: Entity, column: ListColumn): ToggleCellState {
  const property = column.property ?? ''
  const raw = entity.properties?.[property]
  const value = raw === undefined || raw === null || raw === '' ? column.off : String(raw)
  const isOn = value === column.on
  const isOther = !isOn && value !== column.off
  const target = entity._toggles?.[property]
  return {
    hidden: isOther && column.other === 'hidden',
    checked: isOn || (isOther && column.other === 'on'),
    disabled: target === undefined,
    target,
  }
}
