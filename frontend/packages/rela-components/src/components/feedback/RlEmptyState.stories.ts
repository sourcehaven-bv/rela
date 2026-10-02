import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlEmptyState from './RlEmptyState.vue'
import RlButton from '../../components/common/RlButton.vue'

const meta: Meta<typeof RlEmptyState> = {
  title: 'Feedback/Empty state',
  component: RlEmptyState,
  parameters: { layout: 'padded' },
  args: { title: 'No tasks yet', description: 'Create the first task to get this board started.' },
}
export default meta
type Story = StoryObj<typeof RlEmptyState>

/** Nothing has been created yet, so the words invite the first one. */
export const Playground: Story = {
  render: (args) => ({
    components: { RlEmptyState, RlButton },
    setup: () => ({ args }),
    template: `
      <RlEmptyState v-bind="args">
        <template #actions>
          <RlButton variant="primary" icon="plus">New task</RlButton>
        </template>
      </RlEmptyState>
    `,
  }),
}

/**
 * A filter matched nothing. Different case, different words: the way out is
 * to widen the filter, not to create something.
 */
export const NoResults: Story = {
  render: () => ({
    components: { RlEmptyState, RlButton },
    template: `
      <RlEmptyState
        icon="search"
        title="No tasks match these filters"
        description="Try removing a filter or searching for something broader."
      >
        <template #actions>
          <RlButton variant="secondary">Clear filters</RlButton>
        </template>
      </RlEmptyState>
    `,
  }),
}

/** The small size, for an empty section inside a page rather than a whole page. */
export const Small: Story = {
  args: { size: 'sm', title: 'No attachments', description: undefined, icon: 'paperclip' },
}
