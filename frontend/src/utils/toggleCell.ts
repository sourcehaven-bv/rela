import type { Entity, ListColumn } from '@/types'

/**
 * The state of a list toggle cell (TKT-M0AT73B). `hidden` renders nothing;
 * otherwise `checked` is the box and `target` the value a click writes.
 */
export interface ToggleCellState {
  hidden: boolean
  checked: boolean
  disabled: boolean
  target: unknown
}

/** Whether a list column renders as a toggle. */
export function isToggleColumn(column: ListColumn | undefined): boolean {
  return column?.widget === 'toggle' || column?.widget === 'switch'
}

/**
 * Resolves one row's toggle. A boolean column has no `on`/`off` and writes
 * true/false. An enum value that is neither follows `other:`: hidden shows
 * nothing, on shows it checked, disabled shows it locked and unchecked. A
 * click on a checked box always writes `off`.
 *
 * The control is disabled when the row's `_actions` refuse an update, its
 * `_fields` mark the property read-only, or the target option is refused (a
 * transition guard), or the type's declared `moves` give no move from the
 * current value to the target. These are UI hints; the server re-checks the
 * PATCH.
 */
export function toggleCellState(
  entity: Entity,
  column: ListColumn,
  moves?: Record<string, string[]>
): ToggleCellState {
  const prop = column.property ?? ''
  const isBool = column.on === undefined && column.off === undefined
  const onValue: unknown = isBool ? true : column.on
  const offValue: unknown = isBool ? false : column.off
  const raw = entity.properties?.[prop]
  // A field withheld by `visible:` has no value to show or flip.
  if (entity._redacted?.includes(prop)) {
    return { hidden: true, checked: false, disabled: true, target: undefined }
  }
  let checked: boolean
  let other = false
  if (raw === onValue) checked = true
  else if (raw === offValue || raw === undefined || raw === null || raw === '') checked = false
  else {
    other = true
    checked = column.other === 'on'
    if (column.other === 'hidden')
      return { hidden: true, checked: false, disabled: true, target: undefined }
  }
  const target = checked ? offValue : onValue
  const field = entity._fields?.[prop]
  const disabled =
    (other && column.other === 'disabled') ||
    entity._actions?.update === false ||
    field?.writable === false ||
    (typeof target === 'string' && field?.options?.[target] === false) ||
    (field?.options === undefined && moves !== undefined && typeof target === 'string' && !moveAllowed(moves, raw, target))
  return { hidden: false, checked, disabled, target }
}

// moveAllowed reports whether a state machine lets `from` move to `to`. An
// unset value is a create-time state the machine does not constrain here.
function moveAllowed(moves: Record<string, string[]>, from: unknown, to: string): boolean {
  if (from === undefined || from === null || from === '') return true
  return (moves[String(from)] ?? []).includes(to)
}
