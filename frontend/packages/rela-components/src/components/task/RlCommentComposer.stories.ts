import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlCommentComposer from './RlCommentComposer.vue'

const meta: Meta<typeof RlCommentComposer> = {
  title: 'Task/Comment composer',
  component: RlCommentComposer,
  parameters: { layout: 'padded' },
  argTypes: { submitting: { control: 'boolean' } },
  args: {
    placeholder: 'Add a comment...',
    label: 'Add a comment',
    submitLabel: 'Comment',
    cancelLabel: 'Cancel',
    submitting: false,
  },
}
export default meta
type Story = StoryObj<typeof RlCommentComposer>

/**
 * Type into the field to reveal the actions. Submitting clears it; the text
 * is emitted, not stored. Escape cancels.
 */
export const Playground: Story = {}

/** While a comment is posting the submit button shows its pending state. */
export const Submitting: Story = { args: { submitting: true } }
