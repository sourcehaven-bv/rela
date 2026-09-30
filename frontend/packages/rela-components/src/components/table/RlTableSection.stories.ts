import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, fn, userEvent } from 'storybook/test'
import RlTableSection from './RlTableSection.vue'
import type { Task } from '../../types'
import { storyComponent } from '../storyGeneric'
import { tableColumns, tableSections } from '../../fixtures'

/* The section is generic over its item; `Task` is this library's demo row. */
const meta: Meta<typeof RlTableSection<Task>> = {
  title: 'Table/Section',
  component: storyComponent(RlTableSection),
  parameters: { layout: 'padded' },
  argTypes: { showHeaderRow: { control: 'boolean' } },
  args: {
    section: tableSections[0],
    columns: tableColumns,
    showHeaderRow: true,
    addLabel: 'Add Task',
  },
}
export default meta
type Story = StoryObj<typeof RlTableSection<Task>>

export const Playground: Story = {}

export const WithoutHeaderRow: Story = { args: { showHeaderRow: false } }

/** Sections start closed when the fixture marks them collapsed. */
export const Collapsed: Story = {
  args: { section: { ...tableSections[0], collapsed: true } },
}

/**
 * The disclosure reports each change through `collapse`, so a caller can
 * remember the choice and pass it back as `section.collapsed`.
 */
export const ReportsCollapse: Story = {
  args: { onCollapse: fn() },
  play: async ({ args, canvasElement }) => {
    const toggle = canvasElement.querySelector<HTMLButtonElement>('.rl-disclosure')!
    await userEvent.click(toggle)
    await expect(args.onCollapse).toHaveBeenLastCalledWith(tableSections[0], true)
    await expect(canvasElement.querySelector('[role="grid"]')).toBeNull()
    await userEvent.click(toggle)
    await expect(args.onCollapse).toHaveBeenLastCalledWith(tableSections[0], false)
  },
}

/**
 * More fixed columns than the pane can hold. The name keeps its floor and the
 * section grows past the pane, so the table scrolls sideways instead of
 * squeezing the name to nothing under the column beside it.
 */
export const MoreColumnsThanFit: Story = {
  args: {
    columns: Array.from({ length: 6 }, (_, i) =>
      tableColumns.map((column) => ({ ...column, key: `${column.key}-${i}` })),
    ).flat(),
  },
  decorators: [() => ({ template: '<div style="width: 800px; overflow-x: auto"><story /></div>' })],
  play: async ({ canvasElement }) => {
    const name = canvasElement.querySelector<HTMLElement>('.rl-table-row__name')!
    await expect(name.getBoundingClientRect().width).toBeGreaterThanOrEqual(240)
  },
}
