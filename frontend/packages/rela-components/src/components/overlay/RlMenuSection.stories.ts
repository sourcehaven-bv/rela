import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import RlMenuSection from './RlMenuSection.vue'
import RlMenu from './RlMenu.vue'
import RlMenuItem from './RlMenuItem.vue'
import RlMenuSeparator from './RlMenuSeparator.vue'
import RlAvatar from '../data/RlAvatar.vue'
import RlIcon from '../common/RlIcon.vue'

const meta: Meta<typeof RlMenuSection> = {
  title: 'Overlay/Menu section',
  component: RlMenuSection,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlMenuSection>

const menuStyle =
  'width:240px; padding:4px; border:1px solid var(--rl-color-border); border-radius:var(--rl-radius-md); background:var(--rl-color-bg)'

/**
 * Who you are signed in as, above the rows that act on it.
 *
 * The padding matches a row's, so the name starts on the same vertical line as
 * the labels below it. That alignment is the reason this is a component rather
 * than a div at each call site.
 */
export const Playground: Story = {
  args: { label: 'Hanna Bakker', lines: ['hanna@sourcehaven.example', 'Sourcehaven BV'] },
  render: (args) => ({
    components: { RlMenuSection, RlMenuItem, RlMenuSeparator },
    setup: () => ({ args }),
    template: `
      <div role="menu" style="${menuStyle}">
        <RlMenuSection v-bind="args" />
        <RlMenuSeparator />
        <RlMenuItem icon="user" href="#profile">Profile</RlMenuItem>
        <RlMenuItem icon="settings" href="#account">Account</RlMenuItem>
      </div>
    `,
  }),
}

/** A label over a group of rows, which is the other thing this is for. */
export const GroupLabel: Story = {
  args: { label: 'Sort by' },
  render: (args) => ({
    components: { RlMenuSection, RlMenuItem },
    setup: () => ({ args }),
    template: `
      <div role="menu" style="${menuStyle}">
        <RlMenuSection v-bind="args" />
        <RlMenuItem current>Last updated</RlMenuItem>
        <RlMenuItem>Title</RlMenuItem>
      </div>
    `,
  }),
}

/** Without an organisation, or any second line. The block is then one name. */
export const LabelOnly: Story = {
  args: { label: 'Hanna Bakker' },
  render: Playground.render,
}

/**
 * The default slot, for a block that needs markup rather than lines of text.
 * The padding still comes from the component.
 */
export const CustomContent: Story = {
  render: () => ({
    components: { RlMenuSection, RlMenuItem, RlMenuSeparator, RlAvatar },
    template: `
      <div role="menu" style="${menuStyle}">
        <RlMenuSection>
          <div style="display:flex; align-items:center; gap:8px">
            <RlAvatar name="Hanna Bakker" size="sm" decorative />
            <span style="font-weight:500">Hanna Bakker</span>
          </div>
        </RlMenuSection>
        <RlMenuSeparator />
        <RlMenuItem icon="sign-out" href="#out">Sign out</RlMenuItem>
      </div>
    `,
  }),
}

/**
 * The account menu, assembled: the recipe rela follows.
 *
 * Every row is a link, because each one goes to a page the login proxy owns. A
 * real `<a>` can be middle-clicked and is announced as going somewhere, which
 * a button handler cannot be.
 *
 * Sign out stays on the default tone. `tone="danger"` is for destructive
 * entries such as Delete; signing out is disruptive but never a mistake worth
 * a warning colour, and the separator already carries the grouping.
 */
export const AccountMenu: Story = {
  render: () => ({
    components: { RlMenu, RlMenuSection, RlMenuItem, RlMenuSeparator, RlAvatar, RlIcon },
    template: `
      <div style="padding-bottom:280px">
        <RlMenu align="start" placement="bottom">
          <template #trigger="{ toggle, attrs }">
            <button
              type="button"
              v-bind="attrs"
              @click="toggle"
              style="display:flex; align-items:center; gap:8px; width:220px; padding:6px 8px; border:none; border-radius:var(--rl-radius-md); background:transparent; font-family:inherit; text-align:left; cursor:pointer"
            >
              <RlAvatar name="Hanna Bakker" size="sm" decorative />
              <span style="flex:1; min-width:0">
                <span style="display:block; font-size:13px; color:var(--rl-color-text)">Hanna Bakker</span>
                <span style="display:block; font-size:11px; color:var(--rl-color-text-subtle)">Sourcehaven BV</span>
              </span>
              <RlIcon name="chevron-down" :size="14" aria-hidden="true" />
            </button>
          </template>

          <RlMenuSection
            label="Hanna Bakker"
            :lines="['hanna@sourcehaven.example', 'Sourcehaven BV']"
          />
          <RlMenuSeparator />
          <RlMenuItem icon="user" href="#profile">Profile</RlMenuItem>
          <RlMenuItem icon="settings" href="#account">Account</RlMenuItem>
          <RlMenuItem icon="organization" href="#orgs">Switch org</RlMenuItem>
          <RlMenuSeparator />
          <RlMenuItem icon="sign-out" href="#sign-out">Sign out</RlMenuItem>
        </RlMenu>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('button', { name: /Hanna Bakker/ }))

    /*
     * The panel is teleported to the body, so it is outside the canvas.
     */
    const menu = within(document.body).getByRole('menu')

    /* The section is read as content, and is not one of the rows. */
    await expect(within(menu).getByText('hanna@sourcehaven.example')).toBeInTheDocument()
    const rows = within(menu).getAllByRole('menuitem')
    await expect(rows).toHaveLength(4)

    /*
     * Arrow keys walk the rows only. Opening lands on the first row rather
     * than on the name above it, and the section is never a stop.
     */
    await expect(rows[0]).toHaveFocus()
    await userEvent.keyboard('{ArrowUp}')
    await expect(rows[3]).toHaveFocus()
    await expect(rows[3]).toHaveAccessibleName(/Sign out/)

    /* Every row navigates, so every row is a link. */
    for (const row of rows) await expect(row.tagName).toBe('A')
  },
}
