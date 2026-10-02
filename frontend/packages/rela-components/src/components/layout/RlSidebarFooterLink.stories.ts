import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlSidebarFooterLink from './RlSidebarFooterLink.vue'
import { iconNames } from '../common/icons'

const meta: Meta<typeof RlSidebarFooterLink> = {
  title: 'Sidebar/Footer link',
  component: RlSidebarFooterLink,
  parameters: { layout: 'padded' },
  argTypes: { icon: { control: { type: 'select' }, options: iconNames } },
  args: { label: 'Help docs', icon: 'help' },
  decorators: [
    () => ({
      template:
        '<div style="width:260px; background:var(--rl-color-bg-sunken); padding:12px"><story /></div>',
    }),
  ],
}
export default meta
type Story = StoryObj<typeof RlSidebarFooterLink>

export const Playground: Story = {}
