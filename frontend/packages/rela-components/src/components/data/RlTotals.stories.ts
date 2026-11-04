import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, within } from 'storybook/test'
import RlTotals from './RlTotals.vue'

const meta: Meta<typeof RlTotals> = {
  title: 'Data/Totals',
  component: RlTotals,
  parameters: { layout: 'padded' },
  args: {
    label: 'Quote totals',
    rows: [
      { label: 'Subtotal', value: '€ 3.594,00' },
      { label: 'VAT', value: '€ 726,54' },
      { label: 'Total', value: '€ 4.320,54', emphasis: true },
    ],
  },
}
export default meta
type Story = StoryObj<typeof RlTotals>

/**
 * Values arrive formatted: the app knows the currency and the locale. The
 * figures are tabular and right-aligned so the digits stand under each other.
 */
export const Playground: Story = {
  play: async ({ canvasElement }) => {
    const total = within(canvasElement).getByText('€ 4.320,54')
    await expect(total.closest('.rl-totals__row--emphasis')).not.toBeNull()
  },
}
