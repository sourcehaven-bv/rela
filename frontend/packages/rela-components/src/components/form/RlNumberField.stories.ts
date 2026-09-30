import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlNumberField from './RlNumberField.vue'

const meta: Meta<typeof RlNumberField> = {
  title: 'Form/Number field',
  component: RlNumberField,
  parameters: { layout: 'padded' },
  args: { label: 'Estimate', min: 0, step: 0.5 },
  render: (args) => ({
    components: { RlNumberField },
    setup: () => ({ args, value: ref(args.modelValue) }),
    template: '<RlNumberField v-bind="args" v-model="value" style="max-width:240px" />',
  }),
}
export default meta
type Story = StoryObj<typeof RlNumberField>

/** An empty box models as `undefined`, which is distinct from a deliberate 0. */
export const Playground: Story = {}

/** The unit sits outside the box so it is never mistaken for part of the value. */
export const WithSuffix: Story = { args: { modelValue: 4, suffix: 'hours' } }

export const WithRange: Story = {
  args: { label: 'Progress', modelValue: 40, min: 0, max: 100, step: 5, suffix: '%' },
}
