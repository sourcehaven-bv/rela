import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { h } from 'vue'
import RlRelatedRow from './RlRelatedRow.vue'
import RlIcon from '../common/RlIcon.vue'
import type { RelatedItem } from './types'
import { storyComponent } from '../storyGeneric'

const meta: Meta<typeof RlRelatedRow<RelatedItem>> = {
  title: 'Data/Related row',
  component: storyComponent(RlRelatedRow),
  parameters: { layout: 'padded' },
  args: {
    item: {
      id: 's2',
      title: 'Give awesome feedback on UX for Atlas Projects',
      icon: () => h(RlIcon, { name: 'progress' }),
      iconLabel: 'In progress',
      meta: [
        { id: 'due', label: 'Due', value: 'Aug 27' },
        { id: 'assignee', label: 'Assigned to', value: 'Jeroen' },
      ],
    },
  },
}
export default meta
type Story = StoryObj<typeof RlRelatedRow<RelatedItem>>

export const Playground: Story = {}

/** A long title truncates; the values at the end keep their place. */
export const LongTitle: Story = {
  args: {
    item: {
      id: 'long',
      title: 'A title long enough to run past the values at the end of the row and be cut off with an ellipsis',
      meta: [{ id: 'due', label: 'Due', value: 'Aug 27', tone: 'warning' }],
    },
  },
  decorators: [() => ({ template: '<div style="max-width:420px"><story /></div>' })],
}

/** A static row: plain text, for a record already shown in full here. */
export const Static: Story = {
  args: {
    item: {
      id: 'static',
      title: 'Book the venue',
      as: 'span',
      meta: [{ id: 'assignee', label: 'Assigned to', value: 'Sam' }],
    },
  },
}
