import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, waitFor, within } from 'storybook/test'
import { ref } from 'vue'
import RlAppShell from './RlAppShell.vue'
import RlSidebar from './RlSidebar.vue'
import RlPageHeader from './RlPageHeader.vue'
import RlBoard from '../board/RlBoard.vue'
import { boardSections, navGroups } from '../../fixtures'

const meta: Meta<typeof RlAppShell> = {
  title: 'Layout/App shell',
  component: RlAppShell,
  parameters: { layout: 'fullscreen' },
  argTypes: { navOpen: { control: 'boolean' }, panelOpen: { control: 'boolean' } },
}
export default meta
type Story = StoryObj<typeof RlAppShell>

const filler = `<div style="padding:24px">Main content</div>`

/** Sidebar plus main content. */
export const TwoPane: Story = {
  render: () => ({
    components: { RlAppShell, RlSidebar, RlPageHeader },
    setup: () => ({ navOpen: ref(false), groups: navGroups }),
    template: `
      <RlAppShell v-model:nav-open="navOpen">
        <template #sidebar>
          <RlSidebar workspace-name="Atlas Projects" :groups="groups" active-id="atlas" @close="navOpen = false" />
        </template>
        <RlPageHeader title="Atlas for Sourcehaven" status-label="On track" @open-nav="navOpen = true" />
        ${filler}
      </RlAppShell>
    `,
  }),
}

/**
 * A resizable sidebar. Opt-in: `sidebarWidth` is what turns the handle on,
 * because a width that can change has to live somewhere, and where it lives
 * is the application's decision.
 */
export const ResizableSidebar: Story = {
  render: () => ({
    components: { RlAppShell, RlSidebar, RlPageHeader },
    setup: () => ({ navOpen: ref(false), groups: navGroups, sidebarWidth: ref(260) }),
    template: `
      <RlAppShell
        v-model:nav-open="navOpen"
        v-model:sidebar-width="sidebarWidth"
        :sidebar-default-width="260"
      >
        <template #sidebar>
          <RlSidebar workspace-name="Atlas Projects" :groups="groups" active-id="atlas" @close="navOpen = false" />
        </template>
        <RlPageHeader title="Atlas for Sourcehaven" status-label="On track" @open-nav="navOpen = true" />
        ${filler}
      </RlAppShell>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const handle = canvas.getByRole('separator', { name: 'Resize sidebar' })
    const sidebar = canvasElement.querySelector('.rl-sidebar') as HTMLElement

    await expect(Math.round(sidebar.getBoundingClientRect().width)).toBe(260)

    handle.focus()
    await userEvent.keyboard('{PageUp}')

    // The shell binds the width it reports, so the pane actually moves.
    await waitFor(() =>
      expect(Math.round(sidebar.getBoundingClientRect().width)).toBe(324),
    )
  },
}

/**
 * Without `sidebarWidth` there is no handle at all, so every existing shell
 * is unchanged rather than quietly gaining a draggable edge.
 */
export const NotResizableByDefault: Story = {
  render: TwoPane.render,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.queryByRole('separator', { name: 'Resize sidebar' })).toBeNull()
  },
}

/** Sidebar, list and right-hand panel. */
export const ThreePane: Story = {
  render: () => ({
    components: { RlAppShell, RlSidebar, RlPageHeader },
    setup: () => ({ navOpen: ref(false), groups: navGroups }),
    template: `
      <RlAppShell v-model:nav-open="navOpen" panel-open>
        <template #sidebar>
          <RlSidebar workspace-name="Atlas Projects" :groups="groups" active-id="atlas" @close="navOpen = false" />
        </template>
        <RlPageHeader title="Atlas for Sourcehaven" @open-nav="navOpen = true" />
        ${filler}
        <template #panel>
          <div style="padding:24px; border-left:1px solid var(--rl-color-border); height:100%">Detail panel</div>
        </template>
      </RlAppShell>
    `,
  }),
}

/**
 * On a phone the sidebar is an off-canvas drawer. Escape or a tap on the
 * scrim closes it, and the page behind it does not scroll.
 */
export const DrawerOpen: Story = {
  globals: { viewport: { value: 'phone' } },
  render: () => ({
    components: { RlAppShell, RlSidebar, RlPageHeader },
    setup: () => ({ navOpen: ref(true), groups: navGroups }),
    template: `
      <RlAppShell v-model:nav-open="navOpen">
        <template #sidebar>
          <RlSidebar workspace-name="Atlas Projects" :groups="groups" active-id="atlas" @close="navOpen = false" />
        </template>
        <RlPageHeader title="Atlas for Sourcehaven" @open-nav="navOpen = true" />
        ${filler}
      </RlAppShell>
    `,
  }),
}

/**
 * An open overlay panel covers the right of the content, so the content owes
 * that much trailing space: without it the last board column sits under the
 * panel with no way to scroll out from under it.
 *
 * The shell publishes the panel's width as `--rl-overlay-inset` on `main`,
 * and the right page gutter adds it, so every scroll container that pads from
 * that gutter gains the room at once. The board is the pressing case because
 * its horizontal track lives inside the board, out of reach of padding on
 * `main`.
 */
export const OverlayPanelLeavesRoomToScroll: Story = {
  play: async ({ canvasElement }) => {
    const board = canvasElement.querySelector('.rl-board') as HTMLElement
    const panel = canvasElement.querySelector('.rl-app-shell__panel') as HTMLElement

    /*
     * Asserted on the scroll range rather than on the computed padding: a
     * flex row does not always carry its end padding into the scrollable
     * area, and it is the reachability that the user feels, not the value.
     */
    board.scrollLeft = board.scrollWidth
    await waitFor(() => expect(board.scrollLeft).toBeGreaterThan(0))

    const lastColumn = [...board.querySelectorAll('.rl-board-column')].pop() as HTMLElement
    await expect(lastColumn.getBoundingClientRect().right).toBeLessThanOrEqual(
      panel.getBoundingClientRect().left,
    )
  },
  render: () => ({
    components: { RlAppShell, RlSidebar, RlPageHeader, RlBoard },
    setup: () => ({ navOpen: ref(false), groups: navGroups, sections: boardSections }),
    template: `
      <RlAppShell v-model:nav-open="navOpen" panel-open panel-mode="overlay">
        <template #sidebar>
          <RlSidebar workspace-name="Atlas Projects" :groups="groups" active-id="atlas" @close="navOpen = false" />
        </template>
        <RlPageHeader title="Atlas for Sourcehaven" @open-nav="navOpen = true" />
        <RlBoard :sections="sections" />
        <template #panel>
          <div style="width:var(--rl-detail-panel-width); max-width:100%; height:100%;
                      background:var(--rl-color-bg); border-left:1px solid var(--rl-color-border);
                      padding:24px; box-sizing:border-box">
            Detail panel
          </div>
        </template>
      </RlAppShell>
    `,
  }),
}
