import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlTextField from './RlTextField.vue'

const meta: Meta<typeof RlTextField> = {
  title: 'Form/Text field',
  component: RlTextField,
  parameters: { layout: 'padded' },
  argTypes: {
    size: { control: { type: 'inline-radio' }, options: ['sm', 'md', 'lg'] },
    type: { control: { type: 'select' }, options: ['text', 'email', 'url', 'tel', 'password'] },
  },
  args: { label: 'Task name', placeholder: 'Name this task', modelValue: '' },
  render: (args) => ({
    components: { RlTextField },
    setup: () => ({ args, value: ref(args.modelValue ?? '') }),
    template: '<RlTextField v-bind="args" v-model="value" style="max-width:360px" />',
  }),
}
export default meta
type Story = StoryObj<typeof RlTextField>

export const Playground: Story = {}

/** A hint explains the field before anything has gone wrong. */
export const WithHint: Story = {
  args: { hint: 'Shown on the board and in search results.' },
}

/**
 * The error replaces nothing: the hint stays, and both are announced with the
 * error read first.
 */
export const WithError: Story = {
  args: {
    modelValue: 'x',
    hint: 'Shown on the board and in search results.',
    error: 'Use at least 3 characters.',
  },
}

export const Required: Story = { args: { required: true } }

export const Disabled: Story = { args: { modelValue: 'Locked value', disabled: true } }

/** The three sizes line up with the button scale. */
export const Sizes: Story = {
  render: () => ({
    components: { RlTextField },
    setup: () => ({ a: ref(''), b: ref(''), c: ref('') }),
    template: `
      <div style="display:flex; flex-direction:column; gap:16px; max-width:360px">
        <RlTextField v-model="a" label="Small" size="sm" placeholder="Small" />
        <RlTextField v-model="b" label="Medium" size="md" placeholder="Medium" />
        <RlTextField v-model="c" label="Large" size="lg" placeholder="Large" />
      </div>
    `,
  }),
}

/** A label can be hidden where the surrounding context already names the field. */
export const LabelHidden: Story = {
  args: { labelHidden: true, label: 'Search tasks', placeholder: 'Search tasks' },
}
