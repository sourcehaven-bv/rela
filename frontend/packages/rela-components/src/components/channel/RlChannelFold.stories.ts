import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, fn, userEvent, within } from 'storybook/test'
import RlChannelFold from './RlChannelFold.vue'
import { provideMessages } from '../../composables/useMessages'
import { NL_MESSAGES } from '../../composables/messages.nl'

const meta: Meta<typeof RlChannelFold> = {
  title: 'Channel/Channel fold',
  component: RlChannelFold,
  parameters: { layout: 'padded' },
  args: { count: 3, labels: ['Claim', 'Progress'], expanded: false, controls: 'run-1', onToggle: fn() },
}
export default meta
type Story = StoryObj<typeof RlChannelFold>

/** Collapsed: the text names the count and the types; no `aria-controls` yet. */
export const Collapsed: Story = {
  play: async ({ args, canvasElement }) => {
    const button = within(canvasElement).getByRole('button', { name: '3 messages: Claim, Progress' })
    await expect(button).toHaveAttribute('aria-expanded', 'false')
    await expect(button).not.toHaveAttribute('aria-controls')
    // The minimum target size of WCAG 2.5.8.
    await expect(button.getBoundingClientRect().height).toBeGreaterThanOrEqual(24)
    await userEvent.click(button)
    await expect(args.onToggle).toHaveBeenCalled()
  },
}

/** Expanded: the same text, `aria-expanded` and `aria-controls` carry the state. */
export const Expanded: Story = {
  args: { expanded: true },
  play: async ({ canvasElement }) => {
    const button = within(canvasElement).getByRole('button', { name: '3 messages: Claim, Progress' })
    await expect(button).toHaveAttribute('aria-expanded', 'true')
    await expect(button).toHaveAttribute('aria-controls', 'run-1')
  },
}

/** One message reads in the singular. */
export const One: Story = {
  args: { count: 1, labels: ['Progress'] },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('button', { name: '1 message: Progress' })).toBeVisible()
  },
}

/** The library texts in Dutch. */
export const Dutch: Story = {
  render: (args) => ({
    components: { RlChannelFold },
    setup() {
      provideMessages(NL_MESSAGES)
      return { args }
    },
    template: '<RlChannelFold v-bind="args" />',
  }),
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('button', { name: '3 berichten: Claim, Progress' })).toBeVisible()
  },
}
