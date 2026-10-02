import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlCommentComposerBox from './RlCommentComposerBox.vue'

const meta: Meta<typeof RlCommentComposerBox> = {
  title: 'Comment/Comment composer box',
  component: RlCommentComposerBox,
  parameters: { layout: 'centered' },
  render: (args) => ({
    components: { RlCommentComposerBox },
    setup: () => ({ args }),
    template:
      '<div style="width:320px; border:1px solid var(--rl-color-border); border-radius:8px; overflow:hidden"><RlCommentComposerBox v-bind="args" /></div>',
  }),
}
export default meta
type Story = StoryObj<typeof RlCommentComposerBox>

/**
 * The dense composer used inside a popover. Enter inserts a line break and
 * Cmd or Ctrl with Enter posts, because a box this small still has to allow
 * more than one line.
 */
export const Playground: Story = {}

/** With a Cancel button, where the composer can be dismissed on its own. */
export const Cancellable: Story = {
  args: { cancellable: true },
}

/** Posting. The button holds its label so the box does not resize mid-post. */
export const Submitting: Story = {
  args: { submitting: true },
}

/** Replying to an existing thread. */
export const Reply: Story = {
  args: { placeholder: 'Reply...', submitLabel: 'Reply' },
}
