import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlBoardColumn from './RlBoardColumn.vue'
import RlTaskCard from './RlTaskCard.vue'
import type { Task } from '../../types'
import { storyComponent } from '../storyGeneric'
import { boardSections } from '../../fixtures'

/* The column is generic over its item; `Task` is this library's demo row. */
const meta: Meta<typeof RlBoardColumn<Task>> = {
  title: 'Board/Column',
  component: storyComponent(RlBoardColumn),
  parameters: { layout: 'padded' },
  args: { section: boardSections[0], addLabel: 'Add task' },
}
export default meta
type Story = StoryObj<typeof RlBoardColumn<Task>>

/**
 * With no `card` slot the column falls back to the title alone, because that
 * is all it can know an item has.
 */
export const Playground: Story = {}

/** The real card comes from the slot, which is where a consumer's own goes. */
export const WithTaskCard: Story = {
  render: (args) => ({
    components: { RlBoardColumn: storyComponent(RlBoardColumn), RlTaskCard },
    setup: () => ({ args }),
    template: `
      <RlBoardColumn v-bind="args">
        <template #card="{ item, selected }">
          <RlTaskCard :task="item" :selected="selected" />
        </template>
      </RlBoardColumn>
    `,
  }),
}

/**
 * Without `emptyLabel` the column draws its heading and then nothing, which
 * reads as a column that failed to load. It also leaves a board that moves
 * cards with nothing to aim a drop at.
 */
export const Empty: Story = {
  args: { section: { id: 'empty', title: 'Backlog', color: 'grey', items: [] } },
}

export const EmptyWithLabel: Story = {
  args: {
    section: { id: 'empty', title: 'Backlog', color: 'grey', items: [] },
    emptyLabel: 'Nothing here yet',
    emptyDescription: 'Drag a task in, or add one below.',
  },
}
