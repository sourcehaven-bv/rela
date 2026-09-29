import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlCallout from './RlCallout.vue'

const meta: Meta<typeof RlCallout> = {
  title: 'Feedback/Callout',
  component: RlCallout,
  parameters: { layout: 'padded' },
  argTypes: {
    tone: { control: { type: 'inline-radio' }, options: ['info', 'success', 'warning', 'danger'] },
  },
  args: { tone: 'info', title: 'Tip' },
  render: (args) => ({
    components: { RlCallout },
    setup: () => ({ args }),
    template: `
      <RlCallout v-bind="args" style="max-width:520px">
        Subtasks roll up to their parent, so closing every subtask closes the parent too.
      </RlCallout>
    `,
  }),
}
export default meta
type Story = StoryObj<typeof RlCallout>

/**
 * An aside inside a body of text. Quieter than a banner and never announced,
 * because it is part of the reading order rather than something that has
 * just happened.
 */
export const Playground: Story = {}

export const Tones: Story = {
  render: () => ({
    components: { RlCallout },
    template: `
      <div style="display:flex; flex-direction:column; gap:12px; max-width:520px">
        <RlCallout tone="info" title="Tip">Subtasks roll up to their parent.</RlCallout>
        <RlCallout tone="warning" title="Careful">Archiving hides a project from search.</RlCallout>
        <RlCallout tone="danger" title="Cannot be undone">Deleting a project deletes its tasks.</RlCallout>
      </div>
    `,
  }),
}
