import type { CalendarDay } from './calendarGrid'

/**
 * The shapes the calendar components share.
 *
 * An event here is already positioned and already formatted. Which day a
 * stored value falls on, what a chip's fields say, and whether this user may
 * move it are all decided before the event reaches the grid — see
 * `calendarGrid.ts` for why the day mapping in particular stays with the app.
 */

/**
 * Chip and legend colours.
 *
 * The same names as `TagColor`, so one palette covers tags, chips and
 * swatches, and an app that colours a source to match a tag gets the same
 * hue in both places.
 */
export type CalendarColor = 'grey' | 'blue' | 'green' | 'amber' | 'red' | 'purple'

/** One label-and-value line on a chip. */
export interface CalendarEventField {
  /** Omitted when the value speaks for itself, such as a lone person name. */
  label?: string
  value: string
}

/**
 * An event placed on the grid.
 *
 * `startDay` and `endDay` are inclusive and equal for a single-day event. They
 * are the event's REAL extent, not clipped to the visible window: a chip needs
 * to know it is a continuation on the first visible day rather than appearing
 * to start there.
 */
export interface CalendarEvent {
  id: string
  summary: string
  startDay: CalendarDay
  /** Inclusive last day. Equals `startDay` for a single-day event. */
  endDay: CalendarDay
  /** `HH:MM` or similar, already formatted. Absent for an all-day event. */
  timeLabel?: string
  color?: CalendarColor
  /** Detail lines under the title, already resolved and formatted. */
  fields?: CalendarEventField[]
  /** Whether this event may be dragged. An affordance, not an authorisation. */
  draggable?: boolean
}

/** A named group of events, shown in the legend and coloured on the grid. */
export interface CalendarSource {
  id: string
  label: string
  color?: CalendarColor
}

/** What travels with a chip drag: the event, and the day it was taken from. */
export interface CalendarDragPayload {
  event: CalendarEvent
  /** The day the grabbed chip was rendered in, which a multi-day drag is
   * relative to. */
  day: CalendarDay
  native: DragEvent
}
