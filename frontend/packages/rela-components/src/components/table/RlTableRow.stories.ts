import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlTableRow from './RlTableRow.vue'
import type { Task } from '../../types'
import { storyComponent } from '../storyGeneric'
import { tableColumns, tableSections } from '../../fixtures'

/* The row is generic over its item; `Task` is this library's demo row. */
const meta: Meta<typeof RlTableRow<Task>> = {
  title: 'Table/Row',
  component: storyComponent(RlTableRow),
  parameters: { layout: 'padded' },
  args: { item: tableSections[0].items[0], columns: tableColumns },
  /* The row uses grid roles, which need a grid ancestor to be valid. */
  decorators: [
    () => ({
      template: '<div role="grid"><div role="rowgroup"><story /></div></div>',
    }),
  ],
}
export default meta
type Story = StoryObj<typeof RlTableRow<Task>>

export const Playground: Story = {}

/** Below 768px the cells stack and each one shows its column name. */
export const Stacked: Story = {
  globals: { viewport: { value: 'phone' } },
}

/** Columns with no value are dropped entirely when the row stacks. */
export const SparseTask: Story = {
  args: { item: { id: 'sparse', title: 'Only a title' }, columns: tableColumns },
  globals: { viewport: { value: 'phone' } },
}
