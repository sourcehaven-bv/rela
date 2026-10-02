import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, within } from 'storybook/test'
import RlMenuItem from './RlMenuItem.vue'
import RlMenuSeparator from './RlMenuSeparator.vue'

const meta: Meta<typeof RlMenuItem> = {
  title: 'Overlay/Menu item',
  component: RlMenuItem,
  parameters: { layout: 'padded' },
  argTypes: { tone: { control: { type: 'inline-radio' }, options: ['default', 'danger'] } },
  args: { icon: 'link', tone: 'default' },
  render: (args) => ({
    components: { RlMenuItem },
    setup: () => ({ args }),
    template: `
      <div role="menu" style="width:200px; padding:4px; border:1px solid #e8e8e6; border-radius:6px">
        <RlMenuItem v-bind="args">Copy link</RlMenuItem>
      </div>
    `,
  }),
}
export default meta
type Story = StoryObj<typeof RlMenuItem>

export const Playground: Story = {}

/** A full menu: icons, a shortcut hint, a separator and a destructive entry. */
export const AllStates: Story = {
  render: () => ({
    components: { RlMenuItem, RlMenuSeparator },
    template: `
      <div role="menu" style="width:220px; padding:4px; border:1px solid #e8e8e6; border-radius:6px">
        <RlMenuItem icon="link" shortcut="C">Copy link</RlMenuItem>
        <RlMenuItem icon="file">Duplicate</RlMenuItem>
        <RlMenuItem icon="maximize-2" disabled>Move to project</RlMenuItem>
        <RlMenuSeparator />
        <RlMenuItem icon="trash-2" tone="danger">Delete task</RlMenuItem>
      </div>
    `,
  }),
}

/**
 * A menu of places rather than actions: each row is a real link, and the one
 * already in effect is marked. The check replaces the shortcut column, and
 * `aria-current` is what carries the state to a screen reader.
 */
export const Places: Story = {
  render: () => ({
    components: { RlMenuItem },
    template: `
      <div role="menu" style="width:220px; padding:4px; border:1px solid #e8e8e6; border-radius:6px">
        <RlMenuItem icon="users" href="/s/crm/">CRM</RlMenuItem>
        <RlMenuItem icon="shield" href="/s/isms/">ISMS</RlMenuItem>
        <RlMenuItem icon="apps" href="/s/projects/" current>Projects</RlMenuItem>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    const current = canvas.getByRole('menuitem', { name: 'Projects' })
    // A link, so it can be opened in a new tab or copied.
    await expect(current).toHaveAttribute('href', '/s/projects/')
    await expect(current).toHaveAttribute('aria-current', 'page')

    // The other rows are links too, and say nothing about being current.
    await expect(canvas.getByRole('menuitem', { name: 'CRM' })).not.toHaveAttribute('aria-current')
  },
}
