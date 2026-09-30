import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlSpinner from './RlSpinner.vue'

const meta: Meta<typeof RlSpinner> = {
  title: 'Common/Spinner',
  component: RlSpinner,
  parameters: { layout: 'padded' },
  argTypes: { size: { control: { type: 'range', min: 12, max: 48, step: 1 } } },
  args: { size: 20, label: 'Loading' },
}
export default meta
type Story = StoryObj<typeof RlSpinner>

export const Playground: Story = {}

/** The spinner takes its colour from the text around it. */
export const Sizes: Story = {
  render: () => ({
    components: { RlSpinner },
    template: `
      <div style="display:flex; gap:16px; align-items:center; color:var(--rl-color-text-muted)">
        <RlSpinner :size="14" />
        <RlSpinner :size="20" />
        <RlSpinner :size="32" />
      </div>
    `,
  }),
}
