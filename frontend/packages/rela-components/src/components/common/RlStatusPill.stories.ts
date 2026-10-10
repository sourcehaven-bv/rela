import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlStatusPill from './RlStatusPill.vue'

const meta: Meta<typeof RlStatusPill> = {
  title: 'Common/Status pill',
  component: RlStatusPill,
  parameters: { layout: 'padded' },
  argTypes: {
    color: { control: { type: 'select' }, options: ['green', 'amber', 'red', 'grey', 'blue', 'purple'] },
  },
  args: { label: 'On track', color: 'green' },
}
export default meta
type Story = StoryObj<typeof RlStatusPill>

export const Playground: Story = {}

export const States: Story = {
  render: () => ({
    components: { RlStatusPill },
    template: `
      <div style="display:flex; gap:12px; flex-wrap:wrap">
        <RlStatusPill label="On track" color="green" />
        <RlStatusPill label="At risk" color="amber" />
        <RlStatusPill label="Off track" color="red" />
        <RlStatusPill label="Not started" color="grey" />
      </div>
    `,
  }),
}
