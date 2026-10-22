import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import { expect, within } from 'storybook/test'
import RlSegmentedControl from './RlSegmentedControl.vue'

const meta: Meta<typeof RlSegmentedControl> = {
  title: 'Data/Segmented control',
  component: RlSegmentedControl,
  parameters: { layout: 'padded' },
  argTypes: { size: { control: { type: 'inline-radio' }, options: ['sm', 'md'] } },
  args: {
    label: 'Calendar range',
    modelValue: 'month',
    size: 'md',
    options: [
      { value: 'day', label: 'Day' },
      { value: 'week', label: 'Week' },
      { value: 'month', label: 'Month' },
    ],
  },
  render: (args) => ({
    components: { RlSegmentedControl },
    setup: () => ({ args, value: ref(args.modelValue) }),
    template: '<RlSegmentedControl v-bind="args" v-model="value" />',
  }),
}
export default meta
type Story = StoryObj<typeof RlSegmentedControl>

/**
 * Distinct from RlViewTabs: tabs switch between panels of content, while this
 * changes a setting that reshapes the view already on screen. It is a radio
 * group, which is what it behaves like, so arrow keys move between options
 * and the group is a single Tab stop.
 */
export const Playground: Story = {}

export const WithIcons: Story = {
  args: {
    label: 'Zoom level',
    modelValue: 'month',
    options: [
      { value: 'week', label: 'Week', icon: 'chart-no-axes-gantt' },
      { value: 'month', label: 'Month', icon: 'kanban' },
      { value: 'quarter', label: 'Quarter', icon: 'table' },
    ],
  },
}

/** Icon-only, where the label becomes the accessible name. */
export const IconOnly: Story = {
  args: {
    label: 'Layout',
    modelValue: 'board',
    iconOnly: true,
    options: [
      { value: 'board', label: 'Board', icon: 'kanban' },
      { value: 'table', label: 'Table', icon: 'table' },
      { value: 'timeline', label: 'Timeline', icon: 'chart-no-axes-gantt' },
    ],
  },
}

export const Small: Story = { args: { size: 'sm' } }

/** An srHint is read after the label, and is part of the aria-label of an icon-only option. */
export const ScreenReaderHint: Story = {
  args: {
    label: 'Layout',
    modelValue: 'board',
    options: [
      { value: 'board', label: 'Board', icon: 'kanban', srHint: ' (default)' },
      { value: 'table', label: 'Table', icon: 'table' },
    ],
  },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('radio', { name: 'Board (default)' })).toBeVisible()
  },
}

export const ScreenReaderHintIconOnly: Story = {
  args: { ...ScreenReaderHint.args, iconOnly: true },
  play: async ({ canvasElement }) => {
    const board = within(canvasElement).getByRole('radio', { name: 'Board (default)' })
    await expect(board).toHaveAttribute('aria-label', 'Board (default)')
    await expect(board.querySelector('.rl-visually-hidden')).toBeNull()
  },
}
