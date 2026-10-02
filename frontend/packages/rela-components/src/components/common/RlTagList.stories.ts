import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlTagList from './RlTagList.vue'

const tags = [
  { id: '1', label: 'Design', color: 'blue' as const },
  { id: '2', label: 'Urgent', color: 'red' as const },
  { id: '3', label: 'Q3', color: 'green' as const },
  { id: '4', label: 'Needs review', color: 'amber' as const },
]

const meta: Meta<typeof RlTagList> = {
  title: 'Common/Tag list',
  component: RlTagList,
  parameters: { layout: 'padded' },
  args: { tags, max: 2 },
}
export default meta
type Story = StoryObj<typeof RlTagList>

/**
 * Shows the first `max` tags and counts the rest, so a tag row stays inside
 * its column instead of painting over the next one.
 */
export const Playground: Story = {}

/** Under the limit, so there is no counter at all. */
export const NoOverflow: Story = {
  args: { tags: tags.slice(0, 2) },
}

/**
 * The hidden tags are named in the counter's tooltip and in its
 * screen-reader text, so the number is never the only way to know what is
 * behind it.
 */
export const HiddenNamed: Story = {
  args: { max: 1 },
}

/** How the same list reads at each limit. */
export const Limits: Story = {
  render: () => ({
    components: { RlTagList },
    setup: () => ({ tags }),
    template: `
      <div style="display:flex; flex-direction:column; gap:12px">
        <RlTagList v-for="n in 4" :key="n" :tags="tags" :max="n" />
      </div>
    `,
  }),
}
