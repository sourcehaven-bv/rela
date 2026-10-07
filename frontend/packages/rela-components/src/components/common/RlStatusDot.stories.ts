import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlStatusDot from './RlStatusDot.vue'

const meta: Meta<typeof RlStatusDot> = {
  title: 'Common/Status dot',
  component: RlStatusDot,
  parameters: { layout: 'padded' },
  argTypes: {
    color: { control: { type: 'select' }, options: ['green', 'amber', 'red', 'grey', 'blue', 'purple'] },
    size: { control: { type: 'range', min: 6, max: 24, step: 1 } },
  },
  args: { color: 'green', size: 8 },
}
export default meta
type Story = StoryObj<typeof RlStatusDot>

export const Playground: Story = {}

export const AllColors: Story = {
  render: () => ({
    components: { RlStatusDot },
    setup: () => ({ colors: ['green', 'amber', 'red', 'grey', 'blue', 'purple'] as const }),
    template: `
      <div style="display:flex; gap:12px; align-items:center">
        <RlStatusDot v-for="c in colors" :key="c" :color="c" />
      </div>
    `,
  }),
}
