import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlAvatarGroup from './RlAvatarGroup.vue'

const people = [
  { name: 'Ada Lovelace' },
  { name: 'Grace Hopper' },
  { name: 'Alan Turing' },
  { name: 'Katherine Johnson' },
  { name: 'Edsger Dijkstra' },
]

const meta: Meta<typeof RlAvatarGroup> = {
  title: 'Data/Avatar group',
  component: RlAvatarGroup,
  parameters: { layout: 'padded' },
  args: { people, max: 3 },
}
export default meta
type Story = StoryObj<typeof RlAvatarGroup>

/**
 * The row is one labelled image, not a series of them: a screen reader reads
 * "Assigned to Ada Lovelace, Grace Hopper, ..." once rather than announcing
 * each avatar separately. Everyone is named, including those behind the "+N".
 */
export const Playground: Story = {}

/** Under the limit, so there is no "+N". */
export const NoOverflow: Story = {
  args: { people: people.slice(0, 3) },
}

/** A single person still gets the group's label. */
export const One: Story = {
  args: { people: people.slice(0, 1) },
}

/** The label leads the announcement, so relabel it to match the field. */
export const CustomLabel: Story = {
  args: { label: 'Watching', max: 2 },
}

export const Sizes: Story = {
  render: () => ({
    components: { RlAvatarGroup },
    setup: () => ({ people, sizes: ['xs', 'sm', 'md', 'lg'] as const }),
    template: `
      <div style="display:flex; flex-direction:column; gap:12px; align-items:flex-start">
        <RlAvatarGroup v-for="size in sizes" :key="size" :people="people" :size="size" />
      </div>
    `,
  }),
}
