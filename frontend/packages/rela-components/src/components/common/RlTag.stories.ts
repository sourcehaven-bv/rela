import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlTag from './RlTag.vue'

const meta: Meta<typeof RlTag> = {
  title: 'Common/Tag',
  component: RlTag,
  parameters: { layout: 'padded' },
  argTypes: {
    color: {
      control: { type: 'select' },
      options: ['grey', 'blue', 'green', 'amber', 'red', 'purple'],
    },
  },
  args: { label: 'Design', color: 'blue' },
}
export default meta
type Story = StoryObj<typeof RlTag>

export const Playground: Story = {}

export const AllColors: Story = {
  render: () => ({
    components: { RlTag },
    setup: () => ({
      colors: ['grey', 'blue', 'green', 'amber', 'red', 'purple'] as const,
    }),
    template: `
      <div style="display:flex; gap:8px; flex-wrap:wrap">
        <RlTag v-for="c in colors" :key="c" :color="c" :label="c" />
      </div>
    `,
  }),
}
