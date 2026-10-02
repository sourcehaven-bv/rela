import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlDateField from './RlDateField.vue'

const meta: Meta<typeof RlDateField> = {
  title: 'Form/Date field',
  component: RlDateField,
  parameters: { layout: 'padded' },
  args: { label: 'Due date' },
  render: (args) => ({
    components: { RlDateField },
    setup: () => ({ args, value: ref(args.modelValue ?? '') }),
    template: '<RlDateField v-bind="args" v-model="value" style="max-width:240px" />',
  }),
}
export default meta
type Story = StoryObj<typeof RlDateField>

/** The native picker, so it is localised and keyboard accessible already. */
export const Playground: Story = { args: { modelValue: '2026-10-15' } }

/**
 * A datetime states its zone. Without it the same stored instant is read
 * differently by everyone who opens the page.
 */
export const WithTime: Story = {
  args: {
    label: 'Starts at',
    withTime: true,
    modelValue: '2026-10-15T09:30',
    timeZoneLabel: 'Europe/Amsterdam',
  },
}

export const WithRange: Story = {
  args: {
    label: 'Delivery date',
    modelValue: '2026-10-15',
    min: '2026-10-01',
    max: '2026-12-31',
    hint: 'Must fall inside the current quarter.',
  },
}
