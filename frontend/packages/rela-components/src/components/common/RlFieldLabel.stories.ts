import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlFieldLabel from './RlFieldLabel.vue'
import RlText from './RlText.vue'

const meta: Meta<typeof RlFieldLabel> = {
  title: 'Typography/Field label',
  component: RlFieldLabel,
  parameters: { layout: 'padded' },
  argTypes: { as: { control: { type: 'select' }, options: ['dt', 'span'] } },
}
export default meta
type Story = StoryObj<typeof RlFieldLabel>

/** In a definition list, paired with the value it describes. */
export const InDefinitionList: Story = {
  render: () => ({
    components: { RlFieldLabel, RlText },
    setup: () => ({
      rows: [
        { label: 'Status', value: 'In progress' },
        { label: 'Assignee', value: 'Jeroen' },
        { label: 'Due date', value: '18 sep, 2026' },
      ],
    }),
    template: `
      <dl style="display:grid; grid-template-columns:120px 1fr; gap:12px 16px; margin:0">
        <template v-for="r in rows" :key="r.label">
          <RlFieldLabel>{{ r.label }}</RlFieldLabel>
          <RlText as="dd">{{ r.value }}</RlText>
        </template>
      </dl>
    `,
  }),
}

export const Standalone: Story = {
  args: { as: 'span' },
  render: (args) => ({
    components: { RlFieldLabel },
    setup: () => ({ args }),
    template: '<RlFieldLabel v-bind="args">Priority</RlFieldLabel>',
  }),
}
