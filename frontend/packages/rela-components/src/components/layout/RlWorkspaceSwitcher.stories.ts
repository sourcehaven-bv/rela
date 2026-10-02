import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import RlWorkspaceSwitcher from './RlWorkspaceSwitcher.vue'
import RlMenu from '../overlay/RlMenu.vue'
import RlMenuItem from '../overlay/RlMenuItem.vue'

const meta: Meta<typeof RlWorkspaceSwitcher> = {
  title: 'Sidebar/Workspace switcher',
  component: RlWorkspaceSwitcher,
  parameters: { layout: 'padded' },
  args: { name: 'Atlas Projects' },
  decorators: [
    () => ({
      template:
        '<div style="width:260px; background:var(--rl-color-bg-sunken); padding:12px"><story /></div>',
    }),
  ],
}
export default meta
type Story = StoryObj<typeof RlWorkspaceSwitcher>

export const Playground: Story = {}

/** The logo slot replaces the default mark. */
export const CustomLogo: Story = {
  render: (args) => ({
    components: { RlWorkspaceSwitcher },
    setup: () => ({ args }),
    template: `
      <RlWorkspaceSwitcher v-bind="args">
        <template #logo>
          <span style="display:grid; place-items:center; width:20px; height:20px; border-radius:4px; background:var(--rl-color-accent); color:#fff; font-size:11px">A</span>
        </template>
      </RlWorkspaceSwitcher>
    `,
  }),
}

/**
 * The spaces switcher, composed rather than built in: this control is the
 * trigger, `RlMenu` is the menu, and `RlMenuItem` with `href` and `current`
 * is a row. Nothing here knows what a space is, which is why the library has
 * no `RlSpaceSwitcher` — the app owns the list, the URLs and the current id.
 *
 * Spread the menu's `attrs` onto the trigger and the ARIA state comes with
 * it; `align="start"` lines the panel up with the sidebar's left edge.
 */
export const SpacesMenu: Story = {
  render: () => ({
    components: { RlWorkspaceSwitcher, RlMenu, RlMenuItem },
    setup: () => ({
      spaces: [
        { id: 'crm', label: 'CRM', icon: 'users', href: '/s/crm/' },
        { id: 'isms', label: 'ISMS', icon: 'shield', href: '/s/isms/' },
        { id: 'projects', label: 'Projects', icon: 'apps', href: '/s/projects/' },
      ] as const,
      currentId: 'projects',
    }),
    template: `
      <RlMenu align="start">
        <template #trigger="{ toggle, attrs }">
          <RlWorkspaceSwitcher name="Atlas Projects" v-bind="attrs" @click="toggle" />
        </template>
        <RlMenuItem
          v-for="space in spaces"
          :key="space.id"
          :icon="space.icon"
          :href="space.href"
          :current="space.id === currentId"
        >{{ space.label }}</RlMenuItem>
      </RlMenu>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const trigger = canvas.getByRole('button', { name: /Atlas Projects/ })

    await expect(trigger).toHaveAttribute('aria-haspopup', 'menu')
    await expect(trigger).toHaveAttribute('aria-expanded', 'false')

    await userEvent.click(trigger)
    await expect(trigger).toHaveAttribute('aria-expanded', 'true')

    /*
     * The panel is teleported to the body, so it is outside the canvas: query
     * the document. The current space is marked, and every row is a link.
     */
    const menu = within(document.body).getByRole('menu')
    const rows = within(menu).getAllByRole('menuitem')
    await expect(rows).toHaveLength(3)
    await expect(within(menu).getByRole('menuitem', { name: 'Projects' })).toHaveAttribute(
      'aria-current',
      'page',
    )
    await expect(within(menu).getByRole('menuitem', { name: 'CRM' })).toHaveAttribute(
      'href',
      '/s/crm/',
    )
  },
}

/**
 * With one space, or none, there is nothing to switch to. `menu: false` drops
 * the chevron and the hover fill, so the header reads as a label rather than
 * offering a control that opens a menu listing only where you already are.
 */
export const NoMenu: Story = {
  args: { menu: false },
}
