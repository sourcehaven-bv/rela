import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, within } from 'storybook/test'
import RlCalendarEventChip from './RlCalendarEventChip.vue'
import type { CalendarDay } from './calendarGrid'
import type { CalendarEvent } from './types'

const day = (d: number): CalendarDay => ({ year: 2026, month: 9, day: d })

const designReview: CalendarEvent = {
  id: 'e1',
  summary: 'Atlas design review',
  startDay: day(15),
  endDay: day(15),
  timeLabel: '09:30',
  color: 'purple',
  fields: [{ label: 'Owner', value: 'Jeroen' }],
}

const meta: Meta<typeof RlCalendarEventChip> = {
  title: 'Calendar/Calendar event chip',
  component: RlCalendarEventChip,
  parameters: { layout: 'padded' },
  decorators: [() => ({ template: '<div style="width:180px"><story /></div>' })],
  args: { event: designReview, day: day(15) },
}
export default meta
type Story = StoryObj<typeof RlCalendarEventChip>

export const Playground: Story = {}

/** An all-day event has no time, so the title starts at the edge. */
export const AllDay: Story = {
  args: {
    event: {
      id: 'e2',
      summary: 'Company offsite',
      startDay: day(15),
      endDay: day(15),
      color: 'green',
    },
  },
}

/**
 * A title too long for one line wraps to two rather than truncating: "Long
 * sprint" losing its second word to an ellipsis is the case the two-line
 * clamp exists for.
 */
export const LongTitle: Story = {
  args: {
    event: {
      id: 'e3',
      summary: 'Sprint planning for the Atlas migration and the CRM cutover',
      startDay: day(15),
      endDay: day(15),
      timeLabel: '10:00',
      color: 'blue',
    },
  },
}

/** Detail lines are pre-formatted by the caller; a label may be left off. */
export const WithFields: Story = {
  args: {
    event: {
      id: 'e4',
      summary: 'ISO27001 audit',
      startDay: day(15),
      endDay: day(15),
      color: 'amber',
      fields: [
        { label: 'Auditor', value: 'T. de Vries' },
        { label: 'Scope', value: 'Annex A controls' },
        { value: 'Amsterdam' },
      ],
    },
  },
}

/**
 * A three-day event drawn on each of its days. Without the arrows these read
 * as three separate events, which is what makes a conference
 * indistinguishable from a daily stand-up.
 */
export const MultiDaySpan: Story = {
  render: () => ({
    components: { RlCalendarEventChip },
    setup() {
      const event = {
        id: 'e5',
        summary: 'ISO27001 audit week',
        startDay: day(14),
        endDay: day(16),
        color: 'amber' as const,
      }
      return { event, days: [day(14), day(15), day(16)] }
    },
    template: `
      <div style="display:flex; gap:8px">
        <div v-for="d in days" :key="d.day" style="width:160px">
          <div style="font-size:11px; color:var(--rl-color-text-muted); margin-bottom:4px">
            {{ d.day }} Sep
          </div>
          <RlCalendarEventChip :event="event" :day="d" />
        </div>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    // The arrow is decorative; the sentence is what a screen reader gets.
    await expect(canvas.getByLabelText('Continues on following days')).toBeVisible()
    await expect(canvas.getByLabelText('Continues before and after this day')).toBeVisible()
    await expect(canvas.getByLabelText('Continued from earlier days')).toBeVisible()
  },
}

/**
 * An event reaching into the period from outside still reads as a
 * continuation on the first visible day, rather than appearing to start
 * there. The extent on the event is the real one; only the days drawn are
 * clipped.
 */
export const ContinuesFromOutsideTheWindow: Story = {
  args: {
    event: {
      id: 'e6',
      summary: 'Long-running migration',
      startDay: { year: 2026, month: 8, day: 28 },
      endDay: { year: 2026, month: 10, day: 4 },
      color: 'grey',
    },
    day: day(15),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(
      canvas.getByLabelText('Continues before and after this day'),
    ).toBeVisible()
  },
}

/** Every chip colour, which is the tag palette. */
export const Colours: Story = {
  render: () => ({
    components: { RlCalendarEventChip },
    setup: () => ({
      colors: ['grey', 'blue', 'green', 'amber', 'red', 'purple'] as const,
      day: day(15),
    }),
    template: `
      <div style="display:flex; flex-direction:column; gap:4px; width:180px">
        <RlCalendarEventChip
          v-for="c in colors"
          :key="c"
          :event="{ id: c, summary: c, startDay: day, endDay: day, color: c }"
          :day="day"
        />
      </div>
    `,
  }),
}

/** Draggable chips take a grab cursor; the rest are plain buttons. */
export const Draggable: Story = {
  args: { event: { ...designReview, draggable: true } },
}
