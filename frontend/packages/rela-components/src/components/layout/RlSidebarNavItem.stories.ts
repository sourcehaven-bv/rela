import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlSidebarNavItem from './RlSidebarNavItem.vue'
import type { NavItem } from '../../types'

const meta: Meta<typeof RlSidebarNavItem> = {
  title: 'Sidebar/Nav item',
  component: RlSidebarNavItem,
  parameters: { layout: 'padded' },
  decorators: [
    () => ({
      template:
        '<div style="width:260px; background:var(--rl-color-bg-sunken); padding:12px"><story /></div>',
    }),
  ],
}
export default meta
type Story = StoryObj<typeof RlSidebarNavItem>

export const Plain: Story = {
  args: { item: { id: 'search', label: 'Search', icon: 'search' } as NavItem },
}

export const WithBadge: Story = {
  args: { item: { id: 'rela', label: 'Rela', initial: 'R' } as NavItem },
}

export const Active: Story = {
  args: {
    item: { id: 'atlas', label: 'Atlas', initial: 'A' } as NavItem,
    activeId: 'atlas',
  },
}

/** An item with children renders a caret and recurses one level deeper. */
export const Nested: Story = {
  args: {
    activeId: 'ux',
    item: {
      id: 'atlas',
      label: 'Atlas',
      initial: 'A',
      expanded: true,
      children: [
        { id: 'ux', label: 'UX research' },
        { id: 'api', label: 'API design' },
      ],
    } as NavItem,
  },
}

/**
 * Rendered as a link. A sidebar that navigates by URL wants a real anchor,
 * which is what gives the user the middle click, the modifier click and the
 * status bar; a button with a programmatic push gives up all three.
 */
export const AsLink: Story = {
  args: {
    item: {
      id: 'atlas',
      label: 'Atlas',
      initial: 'A',
      as: 'a',
      attrs: { href: '#atlas' },
    } as NavItem,
  },
}

/**
 * One list, both kinds of row. The choice is per item because the two are
 * genuinely different things: an entry that goes somewhere is a link, and an
 * entry that runs an action is a button.
 *
 * The parent here is a link, so its caret becomes its own control rather than
 * the row toggling on press: navigating and expanding are separate requests.
 */
export const MixedLinksAndButtons: Story = {
  render: () => ({
    components: { RlSidebarNavItem: RlSidebarNavItem },
    setup: () => ({
      items: [
        {
          id: 'atlas',
          label: 'Atlas',
          initial: 'A',
          as: 'a',
          attrs: { href: '#atlas' },
          expanded: true,
          children: [
            { id: 'ux', label: 'UX research', as: 'a', attrs: { href: '#ux' } },
            { id: 'api', label: 'API design', as: 'a', attrs: { href: '#api' } },
          ],
        },
        { id: 'search', label: 'Search', icon: 'search', as: 'a', attrs: { href: '#search' } },
        { id: 'reindex', label: 'Rebuild index', icon: 'refresh' },
      ] as NavItem[],
    }),
    template: `
      <div style="width:240px">
        <RlSidebarNavItem
          v-for="item in items"
          :key="item.id"
          :item="item"
          active-id="ux"
        />
      </div>
    `,
  }),
}
