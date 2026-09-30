import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlCommentThread from './RlCommentThread.vue'
import { anchoredComments } from '../../fixtures'

const meta: Meta<typeof RlCommentThread> = {
  title: 'Comment/Comment thread',
  component: RlCommentThread,
  parameters: { layout: 'padded' },
  args: { comments: anchoredComments, showAnchor: true },
}
export default meta
type Story = StoryObj<typeof RlCommentThread>

/**
 * The list behind every comment surface. Edit replaces the body in place, so
 * the row keeps its position while it is being changed.
 */
export const Playground: Story = {}

/** Inside a popover, where the anchor is already named by the header. */
export const Dense: Story = {
  args: {
    dense: true,
    showAnchor: false,
    comments: anchoredComments.slice(0, 2),
  },
  render: (args) => ({
    components: { RlCommentThread },
    setup: () => ({ args }),
    template:
      '<div style="width:320px; border:1px solid var(--rl-color-border); border-radius:8px"><RlCommentThread v-bind="args" /></div>',
  }),
}

/** A comment whose target is gone. It is kept, and says so. */
export const Detached: Story = {
  args: { comments: anchoredComments.filter((c) => c.detached) },
}
