import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlSidebarBadge from './RlSidebarBadge.vue'

const meta: Meta<typeof RlSidebarBadge> = {
  title: 'Sidebar/Badge',
  component: RlSidebarBadge,
  parameters: { layout: 'padded' },
  args: { initial: 'R' },
}
export default meta
type Story = StoryObj<typeof RlSidebarBadge>

export const Playground: Story = {}

export const Set: Story = {
  render: () => ({
    components: { RlSidebarBadge },
    setup: () => ({ initials: ['R', 'J', 'T', 'D'] }),
    template: `
      <div style="display:flex; gap:8px">
        <RlSidebarBadge v-for="i in initials" :key="i" :initial="i" />
      </div>
    `,
  }),
}
