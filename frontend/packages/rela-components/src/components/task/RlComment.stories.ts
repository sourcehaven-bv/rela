import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlComment from './RlComment.vue'
import { detailComments } from '../../fixtures'

const meta: Meta<typeof RlComment> = {
  title: 'Task/Comment',
  component: RlComment,
  parameters: { layout: 'padded' },
  args: { comment: detailComments[0] },
}
export default meta
type Story = StoryObj<typeof RlComment>

export const Playground: Story = {}

export const Thread: Story = {
  render: () => ({
    components: { RlComment },
    setup: () => ({ comments: detailComments }),
    template: `
      <div style="display:flex; flex-direction:column; gap:16px">
        <RlComment v-for="c in comments" :key="c.id" :comment="c" />
      </div>
    `,
  }),
}
