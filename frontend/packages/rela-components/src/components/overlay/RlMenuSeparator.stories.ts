import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlMenuSeparator from './RlMenuSeparator.vue'
import RlMenuItem from './RlMenuItem.vue'

const meta: Meta<typeof RlMenuSeparator> = {
  title: 'Overlay/Menu separator',
  component: RlMenuSeparator,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlMenuSeparator>

const menuStyle =
  'width:220px; padding:4px; border:1px solid var(--rl-color-border); border-radius:var(--rl-radius-md); background:var(--rl-color-bg)'

/**
 * Divides groups of menu items. It carries `role="separator"`, so it is
 * announced as a group boundary rather than skipped in silence: a reader who
 * cannot see the rule still learns that "Delete" belongs to a different group
 * from the actions above it.
 */
export const Playground: Story = {
  render: () => ({
    components: { RlMenuSeparator, RlMenuItem },
    template: `
      <div role="menu" style="${menuStyle}">
        <RlMenuItem icon="link">Copy link</RlMenuItem>
        <RlMenuItem icon="edit">Rename</RlMenuItem>
        <RlMenuSeparator />
        <RlMenuItem icon="delete" tone="danger">Delete</RlMenuItem>
      </div>
    `,
  }),
}

/** Several groups in one menu. */
export const MultipleGroups: Story = {
  render: () => ({
    components: { RlMenuSeparator, RlMenuItem },
    template: `
      <div role="menu" style="${menuStyle}">
        <RlMenuItem icon="link">Copy link</RlMenuItem>
        <RlMenuSeparator />
        <RlMenuItem icon="edit">Rename</RlMenuItem>
        <RlMenuItem icon="copy">Duplicate</RlMenuItem>
        <RlMenuSeparator />
        <RlMenuItem icon="delete" tone="danger">Delete</RlMenuItem>
      </div>
    `,
  }),
}
