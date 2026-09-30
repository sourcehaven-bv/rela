import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import { expect, userEvent, within } from 'storybook/test'
import RlCalendarLegend from './RlCalendarLegend.vue'
import { calendarSources } from '../../fixtures'

const meta: Meta<typeof RlCalendarLegend> = {
  title: 'Calendar/Calendar legend',
  component: RlCalendarLegend,
  parameters: { layout: 'padded' },
  args: { sources: calendarSources, hidden: [] },
}
export default meta
type Story = StoryObj<typeof RlCalendarLegend>

export const Playground: Story = {}

/**
 * A hidden source stays legible: it is the control for bringing itself back,
 * so it has to keep reading as its own name. The strike-through carries the
 * state so it survives without colour.
 */
export const WithHidden: Story = {
  args: { hidden: ['meetings'] },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByRole('button', { name: 'Meetings' })).toHaveAttribute(
      'aria-pressed',
      'false',
    )
    await expect(canvas.getByRole('button', { name: 'Tasks' })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
  },
}

/**
 * One source draws nothing: there is nothing to tell apart, and a lone entry
 * beside its own grid is noise. Pass `showSingle` where the name matters
 * anyway.
 */
export const SingleSource: Story = {
  args: { sources: [calendarSources[0]] },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).queryByRole('button')).toBeNull()
  },
}

export const SingleSourceShown: Story = {
  args: { sources: [calendarSources[0]], showSingle: true },
}

/** Toggling is the caller's state; the legend only reports the click. */
export const Interactive: Story = {
  render: () => ({
    components: { RlCalendarLegend },
    setup() {
      const hidden = ref<string[]>([])
      return {
        hidden,
        sources: calendarSources,
        toggle: (id: string) =>
          (hidden.value = hidden.value.includes(id)
            ? hidden.value.filter((h) => h !== id)
            : [...hidden.value, id]),
      }
    },
    template: `
      <RlCalendarLegend :sources="sources" :hidden="hidden" @toggle="toggle" />
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const releases = canvas.getByRole('button', { name: 'Releases' })
    await expect(releases).toHaveAttribute('aria-pressed', 'true')
    await userEvent.click(releases)
    await expect(releases).toHaveAttribute('aria-pressed', 'false')
  },
}
