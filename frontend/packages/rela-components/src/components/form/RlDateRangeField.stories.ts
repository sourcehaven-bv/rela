import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, within } from 'storybook/test'
import { ref } from 'vue'
import RlDateRangeField from './RlDateRangeField.vue'

const meta: Meta<typeof RlDateRangeField> = {
  title: 'Form/Date range field',
  component: RlDateRangeField,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlDateRangeField>

/**
 * Two ends of one value. Its own component rather than a mode on
 * `RlDateField`, because a range is two dates with a rule between them: each
 * end bounds the other and the pair carries one label and one error.
 */
export const Default: Story = {
  render: () => ({
    components: { RlDateRangeField },
    setup: () => ({ range: ref({ start: '2026-09-01', end: '2026-09-30' }) }),
    template: `
      <div style="max-width:440px">
        <RlDateRangeField v-model="range" label="Reporting period" />
        <p style="margin-top:12px; color:var(--rl-color-text-subtle)">{{ range }}</p>
      </div>
    `,
  }),
}

/** With time, and the zone named, for the same reason the single field names it. */
export const WithTime: Story = {
  args: {
    label: 'Maintenance window',
    withTime: true,
    timeZoneLabel: 'CET',
    modelValue: { start: '2026-10-03T22:00', end: '2026-10-04T02:00' },
  },
}

/** One error for the pair, because the range is one decision. */
export const WithError: Story = {
  args: {
    label: 'Reporting period',
    error: 'Pick a period that ends within the current quarter.',
    modelValue: { start: '2026-09-01', end: '2027-04-01' },
  },
}

/**
 * The ends bound each other through `min` and `max`, so the browser's own
 * picker refuses an end before the start. Telling the user off afterwards
 * would be blaming them for something the control allowed.
 */
export const EndsBoundEachOther: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    const start = canvas.getByLabelText('From')
    const end = canvas.getByLabelText('To')

    /* The start cannot pass the end, and the end cannot precede the start. */
    await expect(start).toHaveAttribute('max', '2026-09-30')
    await expect(end).toHaveAttribute('min', '2026-09-01')

    /* One group, so a reader meets the pair as a single named control. */
    await expect(canvas.getByRole('group', { name: 'Reporting period' })).toBeInTheDocument()
  },
  render: () => ({
    components: { RlDateRangeField },
    setup: () => ({ range: ref({ start: '2026-09-01', end: '2026-09-30' }) }),
    template: `
      <div style="max-width:440px">
        <RlDateRangeField v-model="range" label="Reporting period" />
      </div>
    `,
  }),
}
