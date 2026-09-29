import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlSidebarGroupHeader from './RlSidebarGroupHeader.vue'

const meta: Meta<typeof RlSidebarGroupHeader> = {
  title: 'Sidebar/Group header',
  component: RlSidebarGroupHeader,
  parameters: { layout: 'padded' },
  argTypes: { showMenu: { control: 'boolean' } },
  args: { label: 'Initiatives', showMenu: true },
  decorators: [
    () => ({
      template:
        '<div style="width:260px; background:var(--rl-color-bg-sunken); padding:12px"><story /></div>',
    }),
  ],
}
export default meta
type Story = StoryObj<typeof RlSidebarGroupHeader>

export const Playground: Story = {}

export const WithoutMenu: Story = { args: { showMenu: false } }

/** An add control, shown on hover where a pointer can hover. */
export const WithAdd: Story = { args: { addLabel: 'New initiative' } }
