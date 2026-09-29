import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlProgressBar from './RlProgressBar.vue'

const meta: Meta<typeof RlProgressBar> = {
  title: 'Feedback/Progress bar',
  component: RlProgressBar,
  parameters: { layout: 'padded' },
  argTypes: { size: { control: { type: 'inline-radio' }, options: ['sm', 'md'] } },
  args: { value: 62, max: 100, label: 'Sprint progress', showValue: true, size: 'md' },
  render: (args) => ({
    components: { RlProgressBar },
    setup: () => ({ args }),
    template: '<div style="max-width:360px"><RlProgressBar v-bind="args" /></div>',
  }),
}
export default meta
type Story = StoryObj<typeof RlProgressBar>

export const Playground: Story = {}

/**
 * A rollup of statuses in one bar. It reports the whole breakdown as a single
 * string, because reading four separate progress bars would be far more
 * tiring than one summary.
 */
export const Segmented: Story = {
  args: {
    label: 'Atlas mobile redesign',
    showValue: true,
    value: undefined,
    segments: [
      { label: 'done', value: 14, tone: 'success' },
      { label: 'in progress', value: 5, tone: 'accent' },
      { label: 'blocked', value: 2, tone: 'danger' },
      { label: 'to do', value: 9, tone: 'neutral' },
    ],
  },
}

/** The small size, for a bar inside a card or a table row. */
export const Small: Story = {
  args: { size: 'sm', label: undefined, ariaLabel: 'Sprint progress', showValue: false },
}
