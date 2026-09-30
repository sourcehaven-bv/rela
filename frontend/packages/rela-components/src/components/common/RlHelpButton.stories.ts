import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlHelpButton from './RlHelpButton.vue'

const meta: Meta<typeof RlHelpButton> = {
  title: 'Common/HelpButton',
  component: RlHelpButton,
  parameters: { layout: 'padded' },
  args: { title: 'Story points' },
}
export default meta
type Story = StoryObj<typeof RlHelpButton>

export const Playground: Story = {
  render: (args) => ({
    components: { RlHelpButton },
    setup: () => ({ args }),
    template: `
      <RlHelpButton v-bind="args">
        Story points estimate relative effort, not hours. Compare against work
        the team has already finished rather than against a calendar.
      </RlHelpButton>
    `,
  }),
}

/** Beside the field label it explains, which is where it earns its place. */
export const NextToAField: Story = {
  render: () => ({
    components: { RlHelpButton },
    template: `
      <div style="display:flex; align-items:center; gap:4px; font-size:13px; font-weight:500">
        <span>Story points</span>
        <RlHelpButton title="Story points">
          Relative effort, not hours.
        </RlHelpButton>
      </div>
    `,
  }),
}
