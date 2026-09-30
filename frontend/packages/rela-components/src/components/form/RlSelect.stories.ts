import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlSelect from './RlSelect.vue'

const options = [
  { value: 'todo', label: 'To do' },
  { value: 'doing', label: 'In progress' },
  { value: 'review', label: 'In review' },
  { value: 'done', label: 'Done' },
  { value: 'archived', label: 'Archived', disabled: true },
]

const meta: Meta<typeof RlSelect> = {
  title: 'Form/Select',
  component: RlSelect,
  parameters: { layout: 'padded' },
  args: { label: 'Status', options, modelValue: 'doing' },
  render: (args) => ({
    components: { RlSelect },
    setup: () => ({ args, value: ref(args.modelValue ?? '') }),
    template: '<RlSelect v-bind="args" v-model="value" style="max-width:280px" />',
  }),
}
export default meta
type Story = StoryObj<typeof RlSelect>

/** The native control, so it opens as the platform picker on a phone. */
export const Playground: Story = {}

/** A placeholder occupies the empty state and cannot be chosen again. */
export const WithPlaceholder: Story = {
  args: { modelValue: '', placeholder: 'Choose a status', required: true },
}

export const WithError: Story = {
  args: { modelValue: '', placeholder: 'Choose a status', error: 'Pick a status to continue.' },
}
