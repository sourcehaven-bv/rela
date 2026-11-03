/**
 * Maps a row's `_busy` entries to what `RlBusyIndicator` shows, for one
 * channel type. Shared by the board's busy card field and the list's busy
 * column (TKT-TU3XH4), so the two render the same entries the same way.
 */
import type { BusyIndicatorEntry } from 'rela-components/components/feedback/RlBusyIndicator.vue'
import type { BusyChannel } from '@/types/entity'

/**
 * The entries of `busy` whose type is `type`, as indicator entries. The
 * type label falls back to the type name and the label to the state name.
 *
 * Nil: an absent `busy` (the row was served without `_busy`) gives `[]`.
 */
export function toBusyEntries(busy: BusyChannel[] | undefined, type: string): BusyIndicatorEntry[] {
  return (busy ?? [])
    .filter((b) => b.type === type)
    .map((b) => ({ typeLabel: b.type_label || b.type, label: b.label || b.state, since: b.since }))
}
