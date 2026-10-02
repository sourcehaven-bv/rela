import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlCount from './RlCount.vue'

const meta: Meta<typeof RlCount> = {
  title: 'Common/Count',
  component: RlCount,
  parameters: { layout: 'padded' },
  args: { value: 55 },
}
export default meta
type Story = StoryObj<typeof RlCount>

export const Playground: Story = {}

export const BesideATitle: Story = {
  render: () => ({
    components: { RlCount },
    template: `
      <div style="display:flex; align-items:center; gap:8px">
        <span style="font-size:16px; font-weight:600">Backlog</span>
        <RlCount :value="12" />
      </div>
    `,
  }),
}
