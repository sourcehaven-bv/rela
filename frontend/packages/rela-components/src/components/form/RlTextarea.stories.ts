import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlTextarea from './RlTextarea.vue'

const meta: Meta<typeof RlTextarea> = {
  title: 'Form/Textarea',
  component: RlTextarea,
  parameters: { layout: 'padded' },
  args: { label: 'Description', placeholder: 'Describe the work', rows: 3 },
  render: (args) => ({
    components: { RlTextarea },
    setup: () => ({ args, value: ref(args.modelValue ?? '') }),
    template: '<RlTextarea v-bind="args" v-model="value" style="max-width:480px" />',
  }),
}
export default meta
type Story = StoryObj<typeof RlTextarea>

export const Playground: Story = {}

/** The box follows the text instead of scrolling. Type to see it grow. */
export const AutoGrow: Story = {
  args: { autoGrow: true, modelValue: 'The box grows as this text wraps onto more lines.' },
}

/** A live count, announced as a status so it does not interrupt every keystroke. */
export const WithCounter: Story = {
  args: { maxlength: 280, modelValue: 'Counts toward the limit.' },
}

export const WithError: Story = {
  args: { modelValue: '', error: 'A description is required before this can be assigned.' },
}
