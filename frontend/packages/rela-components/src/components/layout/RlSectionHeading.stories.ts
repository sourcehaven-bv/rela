import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlSectionHeading from './RlSectionHeading.vue'

const meta: Meta<typeof RlSectionHeading> = {
  title: 'Layout/Section heading',
  component: RlSectionHeading,
  parameters: { layout: 'padded' },
  argTypes: {
    color: { control: { type: 'select' }, options: ['green', 'amber', 'red', 'grey', 'blue'] },
    size: { control: { type: 'inline-radio' }, options: ['sm', 'md'] },
    level: { control: { type: 'select' }, options: [1, 2, 3, 4, 5, 6] },
  },
  args: { title: 'In progress', color: 'green', count: 5, size: 'md', level: 2 },
}
export default meta
type Story = StoryObj<typeof RlSectionHeading>

export const Playground: Story = {}

export const Variants: Story = {
  render: () => ({
    components: { RlSectionHeading },
    template: `
      <div style="display:flex; flex-direction:column; gap:12px">
        <RlSectionHeading title="Backlog" color="grey" :count="4" />
        <RlSectionHeading title="In progress" color="green" :count="5" />
        <RlSectionHeading title="On Hold" color="red" :count="5" size="sm" />
        <RlSectionHeading title="No count" color="blue" />
      </div>
    `,
  }),
}
