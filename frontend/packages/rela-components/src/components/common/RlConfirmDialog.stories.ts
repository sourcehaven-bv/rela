import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlConfirmDialog from './RlConfirmDialog.vue'
import RlButton from './RlButton.vue'

const meta: Meta<typeof RlConfirmDialog> = {
  title: 'Common/Confirm dialog',
  component: RlConfirmDialog,
  parameters: { layout: 'padded' },
  argTypes: {
    tone: { control: { type: 'inline-radio' }, options: ['default', 'danger'] },
    confirming: { control: 'boolean' },
  },
  args: {
    title: 'Delete this task?',
    description: 'Atlas mobile redesign and its 3 subtasks will be removed. This cannot be undone.',
    confirmLabel: 'Delete',
    cancelLabel: 'Cancel',
    tone: 'danger',
    confirming: false,
  },
}
export default meta
type Story = StoryObj<typeof RlConfirmDialog>

/**
 * Opens focused on Cancel, traps Tab, closes on Escape or a scrim click, and
 * returns focus to the button that opened it.
 */
export const Playground: Story = {
  render: (args) => ({
    components: { RlConfirmDialog, RlButton },
    setup() {
      const open = ref(false)
      return { args, open }
    },
    template: `
      <div>
        <RlButton variant="primary" tone="danger" icon="trash-2" @click="open = true">
          Delete task
        </RlButton>
        <RlConfirmDialog
          v-bind="args"
          :open="open"
          @cancel="open = false"
          @confirm="open = false"
        />
      </div>
    `,
  }),
}

/** A neutral confirmation, for an action that is disruptive but not a deletion. */
export const Neutral: Story = {
  ...Playground,
  args: {
    title: 'Discard your changes?',
    description: 'Your edits to this document will be lost.',
    confirmLabel: 'Discard',
    tone: 'default',
  },
}

/** Shown open so the layout is visible without interaction. */
export const Open: Story = { args: { open: true } }
