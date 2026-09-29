import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import { ref } from 'vue'
import RlSwitch from './RlSwitch.vue'

const meta: Meta<typeof RlSwitch> = {
  title: 'Form/Switch',
  component: RlSwitch,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlSwitch>

/**
 * A setting that takes effect as it is flipped. Not a restyled checkbox: a
 * checkbox means "include this when I submit" and belongs in a form with a
 * button under it, a switch means "this is on now".
 */
export const Default: Story = {
  render: () => ({
    components: { RlSwitch },
    setup: () => ({ on: ref(true) }),
    template: `
      <div style="max-width:420px; display:flex; flex-direction:column; gap:16px">
        <RlSwitch v-model="on" label="Send a weekly digest" hint="Every Monday at 09:00." />
        <RlSwitch label="Require two-factor sign-in" />
        <RlSwitch label="Archived workspace" disabled />
      </div>
    `,
  }),
}

/**
 * A switch that applies immediately is usually waiting on a server. It holds
 * its old position while the request is out rather than flipping
 * optimistically, so it never shows a state the server has not agreed to.
 */
export const Pending: Story = {
  args: { label: 'Public sharing', modelValue: false, pending: true },
}

/**
 * Announced as a switch that is on or off rather than a box that is ticked,
 * which is what `role="switch"` buys: the words match what the control does.
 */
export const TogglesAndAnnounces: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const digest = canvas.getByRole('switch', { name: /weekly digest/ })

    await expect(digest).toHaveAttribute('aria-checked', 'true')

    await userEvent.click(digest)
    await expect(digest).toHaveAttribute('aria-checked', 'false')

    /* Enter flips it too: the ARIA pattern asks for Space and Enter both. */
    digest.focus()
    await userEvent.keyboard('{Enter}')
    await expect(digest).toHaveAttribute('aria-checked', 'true')

    await userEvent.keyboard(' ')
    await expect(digest).toHaveAttribute('aria-checked', 'false')

    /* A disabled switch is not reachable and does not change. */
    const archived = canvas.getByRole('switch', { name: /Archived/ })
    await expect(archived).toBeDisabled()
  },
  render: () => ({
    components: { RlSwitch },
    setup: () => ({ on: ref(true) }),
    template: `
      <div style="max-width:420px; display:flex; flex-direction:column; gap:16px">
        <RlSwitch v-model="on" label="Send a weekly digest" hint="Every Monday at 09:00." />
        <RlSwitch label="Archived workspace" disabled />
      </div>
    `,
  }),
}

/** Pending blocks a second press, so a slow save cannot be queued up twice. */
export const PendingBlocks: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const control = canvas.getByRole('switch', { name: /Public sharing/ })

    await expect(control).toBeDisabled()
    await userEvent.click(control)
    await expect(control).toHaveAttribute('aria-checked', 'false')
  },
  args: { label: 'Public sharing', modelValue: false, pending: true },
}
