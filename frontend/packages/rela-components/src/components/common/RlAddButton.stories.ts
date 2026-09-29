import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlAddButton from './RlAddButton.vue'

const meta: Meta<typeof RlAddButton> = {
  title: 'Common/Add button',
  component: RlAddButton,
  parameters: { layout: 'padded' },
  argTypes: { block: { control: 'boolean' } },
  args: { label: 'Add task', block: false },
}
export default meta
type Story = StoryObj<typeof RlAddButton>

export const Playground: Story = {}

/** `block` fills the width of a board column. */
export const Inline: Story = {
  render: () => ({
    components: { RlAddButton },
    template: `
      <div style="display:flex; flex-direction:column; align-items:flex-start; gap:8px; width:296px">
        <RlAddButton label="Add task" />
        <RlAddButton label="Add section" />
        <RlAddButton label="Add Task" block />
      </div>
    `,
  }),
}
