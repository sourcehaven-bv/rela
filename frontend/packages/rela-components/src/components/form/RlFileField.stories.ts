import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlFileField from './RlFileField.vue'

const meta: Meta<typeof RlFileField> = {
  title: 'Form/File field',
  component: RlFileField,
  parameters: { layout: 'padded' },
  args: { label: 'Attachments' },
  render: (args) => ({
    components: { RlFileField },
    setup: () => ({ args, value: ref<File[]>([]) }),
    template: '<RlFileField v-bind="args" v-model="value" style="max-width:420px" />',
  }),
}
export default meta
type Story = StoryObj<typeof RlFileField>

/**
 * Dragging is an addition, never the only route: the zone is a label for a
 * real file input, so clicking and the keyboard both work.
 */
export const Playground: Story = {}

export const Multiple: Story = {
  args: { multiple: true, hint: 'PDF, PNG or JPG, up to 10 MB each.' },
}

export const WithError: Story = {
  args: { error: 'That file type is not accepted.', accept: '.pdf,.png,.jpg' },
}
