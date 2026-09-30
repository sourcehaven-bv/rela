import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlButtonGroup from './RlButtonGroup.vue'
import RlButton from './RlButton.vue'

const meta: Meta<typeof RlButtonGroup> = {
  title: 'Common/Button group',
  component: RlButtonGroup,
  parameters: { layout: 'padded' },
  argTypes: {
    align: { control: { type: 'inline-radio' }, options: ['start', 'end', 'between'] },
    stacked: { control: 'boolean' },
  },
  args: { align: 'end', stacked: false },
  render: (args) => ({
    components: { RlButtonGroup, RlButton },
    setup: () => ({ args }),
    template: `
      <div style="max-width:420px">
        <RlButtonGroup v-bind="args">
          <RlButton variant="ghost">Cancel</RlButton>
          <RlButton variant="primary">Create</RlButton>
        </RlButtonGroup>
      </div>
    `,
  }),
}
export default meta
type Story = StoryObj<typeof RlButtonGroup>

export const Playground: Story = {}

export const Alignments: Story = {
  render: () => ({
    components: { RlButtonGroup, RlButton },
    template: `
      <div style="display:flex; flex-direction:column; gap:20px; max-width:420px">
        <RlButtonGroup align="start">
          <RlButton variant="ghost">Cancel</RlButton>
          <RlButton variant="primary">Create</RlButton>
        </RlButtonGroup>

        <RlButtonGroup align="end">
          <RlButton variant="ghost">Cancel</RlButton>
          <RlButton variant="primary">Create</RlButton>
        </RlButtonGroup>

        <RlButtonGroup align="between">
          <RlButton variant="ghost" tone="danger">Delete</RlButton>
          <RlButton variant="primary">Save</RlButton>
        </RlButtonGroup>
      </div>
    `,
  }),
}

/** Stacked suits a narrow drawer, where a side-by-side pair would be cramped. */
export const Stacked: Story = {
  render: () => ({
    components: { RlButtonGroup, RlButton },
    template: `
      <div style="max-width:280px">
        <RlButtonGroup stacked>
          <RlButton variant="primary">Create task</RlButton>
          <RlButton variant="secondary">Cancel</RlButton>
        </RlButtonGroup>
      </div>
    `,
  }),
}
