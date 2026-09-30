import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import { computed, ref } from 'vue'
import RlResizer from './RlResizer.vue'
import RlSidebar from './RlSidebar.vue'
import { navGroups } from '../../fixtures'

const meta: Meta<typeof RlResizer> = {
  title: 'Layout/Resizer',
  component: RlResizer,
  parameters: { layout: 'fullscreen' },
}
export default meta
type Story = StoryObj<typeof RlResizer>

/**
 * Beside the sidebar, not inside it. `RlSidebar` takes its width from
 * `--rl-sidebar-width`, so the app rebinds the token and there stays one rule
 * about how wide the sidebar is.
 *
 * The clamp is the caller's and is applied at render, so a narrow window
 * shows a narrow sidebar without overwriting the width that was chosen.
 */
export const Default: Story = {
  render: () => ({
    components: { RlResizer, RlSidebar },
    setup() {
      const width = ref(260)
      // What the app would persist: one number, not one per breakpoint.
      const style = computed(() => ({
        '--rl-sidebar-width': `clamp(200px, ${width.value}px, 40vw)`,
      }))
      return { width, style, navGroups }
    },
    template: `
      <div style="height:100vh; display:flex" :style="style">
        <RlSidebar workspace-name="Atlas Projects" :groups="navGroups" active-id="atlas" />
        <RlResizer
          v-model="width"
          label="Resize sidebar"
          :min="200"
          :max="420"
          :default-value="260"
        />
        <main style="flex:1; padding:24px">
          <p style="font-size:14px; color:var(--rl-color-text-muted)">
            Sidebar width: {{ width }}px. Drag the edge, or focus it and use the arrows.
          </p>
        </main>
      </div>
    `,
  }),
}

/**
 * A drag handle is the easiest control in a layout to leave unreachable, so
 * it is a focusable separator: arrows nudge, Home and End take the ends, and
 * Enter returns to the default.
 */
export const ReachableByKeyboard: Story = {
  render: Default.render,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const handle = canvas.getByRole('separator', { name: 'Resize sidebar' })

    await expect(handle).toHaveAttribute('aria-valuenow', '260')

    handle.focus()
    await expect(handle).toHaveFocus()

    // Right widens a sidebar, because the pane being sized is on the left.
    await userEvent.keyboard('{ArrowRight}')
    await expect(handle).toHaveAttribute('aria-valuenow', '276')

    await userEvent.keyboard('{ArrowLeft}{ArrowLeft}')
    await expect(handle).toHaveAttribute('aria-valuenow', '244')

    await userEvent.keyboard('{End}')
    await expect(handle).toHaveAttribute('aria-valuenow', '420')

    await userEvent.keyboard('{Home}')
    await expect(handle).toHaveAttribute('aria-valuenow', '200')

    await userEvent.keyboard('{Enter}')
    await expect(handle).toHaveAttribute('aria-valuenow', '260')
  },
}

/**
 * The ends hold: nothing past `min` or `max` is reported, so a caller storing
 * the value never has to re-check it.
 */
export const StaysWithinItsEnds: Story = {
  render: Default.render,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const handle = canvas.getByRole('separator', { name: 'Resize sidebar' })

    handle.focus()
    await userEvent.keyboard('{Home}')
    await userEvent.keyboard('{ArrowLeft}{ArrowLeft}{ArrowLeft}')
    await expect(handle).toHaveAttribute('aria-valuenow', '200')

    await userEvent.keyboard('{End}')
    await userEvent.keyboard('{PageUp}{PageUp}')
    await expect(handle).toHaveAttribute('aria-valuenow', '420')
  },
}

/**
 * The grip is what says the edge can be moved. Without it the handle is a 1px
 * line in the same grey as an ordinary border, and the only thing advertising
 * the drag is a cursor that appears once the pointer is already on it.
 */
export const ShowsAGrip: Story = {
  render: Default.render,
  play: async ({ canvasElement }) => {
    const grip = canvasElement.querySelector('.rl-resizer__grip') as HTMLElement
    const line = canvasElement.querySelector('.rl-resizer__line') as HTMLElement

    // Drawn at rest, not only on hover.
    await expect(grip).toBeVisible()

    const gripBox = grip.getBoundingClientRect()
    await expect(Math.round(gripBox.height)).toBe(28)

    // Centred on the same edge as the line, so the two read as one thing.
    const lineBox = line.getBoundingClientRect()
    await expect(Math.abs(gripBox.x + gripBox.width / 2 - (lineBox.x + lineBox.width / 2))).
      toBeLessThanOrEqual(1)

    // Distinct from the border it sits on, or it is not a signal.
    await expect(getComputedStyle(grip).backgroundColor).not.toBe(
      getComputedStyle(line).backgroundColor,
    )
  },
}

/**
 * The width it reports is the width the sidebar takes, so the handle and the
 * pane's edge cannot drift apart.
 */
export const MovesTheSidebar: Story = {
  render: Default.render,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const handle = canvas.getByRole('separator', { name: 'Resize sidebar' })
    const sidebar = canvasElement.querySelector('.rl-sidebar') as HTMLElement

    const before = Math.round(sidebar.getBoundingClientRect().width)
    await expect(before).toBe(260)

    handle.focus()
    await userEvent.keyboard('{PageUp}')

    const after = Math.round(sidebar.getBoundingClientRect().width)
    await expect(after).toBe(324)
  },
}
