import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { computed, ref } from 'vue'
import { expect, userEvent, within } from 'storybook/test'
import RlCalendarGrid from './RlCalendarGrid.vue'
import RlCalendarLegend from './RlCalendarLegend.vue'
import RlButton from '../common/RlButton.vue'
import { shiftAnchor, visibleDays } from './calendarGrid'
import type { CalendarDay, CalendarView } from './calendarGrid'
import type { CalendarEvent } from './types'
import { calendarAnchor, calendarEvents, calendarSources, calendarToday } from '../../fixtures'

const meta: Meta<typeof RlCalendarGrid> = {
  title: 'Calendar/Calendar grid',
  component: RlCalendarGrid,
  parameters: { layout: 'fullscreen' },
  decorators: [() => ({ template: '<div style="padding:24px"><story /></div>' })],
  args: {
    view: 'month' as CalendarView,
    days: visibleDays('month', calendarAnchor),
    anchor: calendarAnchor,
    today: calendarToday,
    events: calendarEvents,
  },
}
export default meta
type Story = StoryObj<typeof RlCalendarGrid>

/** September 2026. The 22nd is today, and carries four events. */
export const Playground: Story = {}

/** A week is one taller row, so a busy day has somewhere to put its events. */
export const WeekView: Story = {
  args: {
    view: 'week',
    days: visibleDays('week', calendarToday),
    anchor: calendarToday,
  },
}

/**
 * A cap collapses the rest of a busy day into a button, so one heavy day
 * cannot set the height of every row in the month.
 */
export const CappedPerDay: Story = {
  args: { maxPerDay: 2 },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    // The 22nd holds four events, two of which the cap hides.
    await expect(canvas.getByRole('button', { name: '+2 more' })).toBeVisible()
  },
}

/** An empty period still draws its cells, rather than collapsing. */
export const NoEvents: Story = {
  args: { events: [] },
}

/** A week starting on Sunday, with the headers following the columns. */
export const SundayWeekStart: Story = {
  args: {
    days: visibleDays('month', calendarAnchor, 'sunday'),
  },
}

/**
 * The whole view, assembled the way an app would: the grid keeps no state, so
 * the period, the hidden sources and the drag all live here.
 */
export const FullView: Story = {
  render: () => ({
    components: { RlCalendarGrid, RlCalendarLegend, RlButton },
    setup() {
      const view = ref<CalendarView>('month')
      const anchor = ref<CalendarDay>(calendarAnchor)
      const hidden = ref<string[]>([])
      const draggingId = ref<string | undefined>()

      // The fixture has no source ids on its events, so they are keyed by
      // colour here. A real app carries the source on the event.
      const sourceOf = (event: CalendarEvent) =>
        event.color === 'purple' ? 'meetings' : event.color === 'green' ? 'releases' : 'tasks'

      const days = computed(() => visibleDays(view.value, anchor.value))
      const events = computed(() =>
        calendarEvents.filter((e) => !hidden.value.includes(sourceOf(e))),
      )
      const title = computed(() =>
        new Intl.DateTimeFormat(undefined, { month: 'long', year: 'numeric' }).format(
          new Date(anchor.value.year, anchor.value.month - 1, anchor.value.day),
        ),
      )

      function toggle(id: string) {
        hidden.value = hidden.value.includes(id)
          ? hidden.value.filter((h) => h !== id)
          : [...hidden.value, id]
      }

      return {
        view,
        anchor,
        days,
        events,
        hidden,
        title,
        draggingId,
        today: calendarToday,
        sources: calendarSources,
        toggle,
        shift: (delta: number) => (anchor.value = shiftAnchor(view.value, anchor.value, delta)),
      }
    },
    template: `
      <div style="display:flex; flex-direction:column; gap:12px">
        <div style="display:flex; align-items:center; gap:12px">
          <RlButton variant="secondary" @click="shift(-1)">Previous</RlButton>
          <strong style="min-width:180px; text-align:center">{{ title }}</strong>
          <RlButton variant="secondary" @click="shift(1)">Next</RlButton>
          <RlCalendarLegend
            style="margin-left:auto"
            :sources="sources"
            :hidden="hidden"
            @toggle="toggle"
          />
        </div>
        <RlCalendarGrid
          :view="view"
          :days="days"
          :anchor="anchor"
          :today="today"
          :events="events"
          :max-per-day="3"
          :dragging-id="draggingId"
          @dragstart="draggingId = $event.event.id"
          @dragend="draggingId = undefined"
        />
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('Atlas design review')).toBeVisible()

    // Hiding a source takes its events off the grid, not just its colour.
    await userEvent.click(canvas.getByRole('button', { name: 'Meetings' }))
    await expect(canvas.queryByText('Atlas design review')).toBeNull()

    await userEvent.click(canvas.getByRole('button', { name: 'Meetings' }))
    await expect(canvas.getByText('Atlas design review')).toBeVisible()
  },
}
