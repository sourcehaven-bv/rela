import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlTaskCard from './RlTaskCard.vue'
import { boardSections } from '../../fixtures'

const meta: Meta<typeof RlTaskCard> = {
  title: 'Board/Task card',
  component: RlTaskCard,
  parameters: { layout: 'padded' },
  decorators: [() => ({ template: '<div style="width:296px"><story /></div>' })],
}
export default meta
type Story = StoryObj<typeof RlTaskCard>

export const Playground: Story = {
  args: { task: boardSections[0].items[0] },
}

/** Tags, assignee, due date and counts are all optional. */
export const Minimal: Story = {
  args: { task: { id: 't', title: 'Write the release notes' } },
}

export const WithEverything: Story = {
  args: {
    task: {
      id: 't',
      title: 'Create awesome UX for Atlas Projects',
      tags: [
        { id: '1', label: 'Design', color: 'blue' },
        { id: '2', label: 'Urgent', color: 'red' },
      ],
      assignee: 'Jeroen',
      dueDate: '18 sep, 2026',
      commentCount: 3,
      subtaskCount: 5,
    },
  },
}
