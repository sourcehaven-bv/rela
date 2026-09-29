import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlBoardColumnCollapsed from './RlBoardColumnCollapsed.vue'
import type { Task } from '../../types'
import { storyComponent } from '../storyGeneric'

const meta: Meta<typeof RlBoardColumnCollapsed<Task>> = {
  title: 'Board/Column (collapsed)',
  component: storyComponent(RlBoardColumnCollapsed),
  parameters: { layout: 'padded' },
  args: {
    section: { id: 'postpone', title: 'Postpone', color: 'grey', count: 3, collapsed: true, items: [] },
  },
  decorators: [() => ({ template: '<div style="height:420px; display:flex"><story /></div>' })],
}
export default meta
type Story = StoryObj<typeof RlBoardColumnCollapsed<Task>>

/** The title runs vertically so the column stays narrow. */
export const Playground: Story = {}
