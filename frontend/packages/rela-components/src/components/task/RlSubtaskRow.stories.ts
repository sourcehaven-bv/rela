import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlSubtaskRow from './RlSubtaskRow.vue'
import { detailSubtasks } from '../../fixtures'

const meta: Meta<typeof RlSubtaskRow> = {
  title: 'Task/Subtask row',
  component: RlSubtaskRow,
  parameters: { layout: 'padded' },
  argTypes: { highlight: { control: 'boolean' } },
  args: { task: detailSubtasks[0], highlight: false },
}
export default meta
type Story = StoryObj<typeof RlSubtaskRow>

export const Playground: Story = {}

export const List: Story = {
  render: () => ({
    components: { RlSubtaskRow },
    setup: () => ({ subtasks: detailSubtasks }),
    template: `
      <div style="border-top:1px solid var(--rl-color-border)">
        <RlSubtaskRow
          v-for="(t, i) in subtasks"
          :key="t.id"
          :task="t"
          :highlight="i === 0"
        />
      </div>
    `,
  }),
}
