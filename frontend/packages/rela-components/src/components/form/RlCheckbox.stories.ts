import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlCheckbox from './RlCheckbox.vue'

const meta: Meta<typeof RlCheckbox> = {
  title: 'Form/Checkbox',
  component: RlCheckbox,
  parameters: { layout: 'padded' },
  args: { label: 'Notify the assignee' },
  render: (args) => ({
    components: { RlCheckbox },
    setup: () => ({ args, value: ref(args.modelValue ?? false) }),
    template: '<RlCheckbox v-bind="args" v-model="value" />',
  }),
}
export default meta
type Story = StoryObj<typeof RlCheckbox>

/** The whole row is the label, so the text is a hit target too. */
export const Playground: Story = {}

export const Checked: Story = { args: { modelValue: true } }

/** For a parent whose children are only partly checked. */
export const Indeterminate: Story = { args: { indeterminate: true, label: 'All subtasks' } }

export const WithHint: Story = {
  args: { hint: 'They will get an email as soon as this is saved.' },
}

export const Disabled: Story = { args: { modelValue: true, disabled: true } }
