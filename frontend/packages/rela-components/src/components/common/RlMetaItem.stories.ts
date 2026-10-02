import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlMetaItem from './RlMetaItem.vue'
import { iconNames } from './icons'

const meta: Meta<typeof RlMetaItem> = {
  title: 'Common/Meta item',
  component: RlMetaItem,
  parameters: { layout: 'padded' },
  argTypes: { icon: { control: { type: 'select' }, options: [undefined, ...iconNames] } },
  args: { icon: 'message-square', label: 3, countLabel: '3 comments' },
}
export default meta
type Story = StoryObj<typeof RlMetaItem>

export const Playground: Story = {}

/**
 * The icon alone does not say what the number counts, so `countLabel` adds
 * that meaning for screen readers without showing it on screen.
 */
export const OnACard: Story = {
  render: () => ({
    components: { RlMetaItem },
    template: `
      <div style="display:flex; gap:12px; align-items:center">
        <RlMetaItem icon="message-square" :label="3" count-label="3 comments" />
        <RlMetaItem icon="git-branch" :label="5" count-label="5 subtasks" />
        <RlMetaItem label="18 sep" />
      </div>
    `,
  }),
}
