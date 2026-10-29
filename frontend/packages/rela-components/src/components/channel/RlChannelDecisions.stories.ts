import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, fn, userEvent, within } from 'storybook/test'
import RlChannelDecisions from './RlChannelDecisions.vue'
import { DECISIONS_BLOCKED_KEY, decisionPendingKey } from './channelFields'
import { provideMessages } from '../../composables/useMessages'
import { NL_MESSAGES } from '../../composables/messages.nl'
import { breachDecisions } from '../../fixtures/channels'

const meta: Meta<typeof RlChannelDecisions> = {
  title: 'Channel/Channel decisions',
  component: RlChannelDecisions,
  parameters: { layout: 'padded' },
  args: { decisions: breachDecisions, onDecide: fn() },
}
export default meta
type Story = StoryObj<typeof RlChannelDecisions>

/** A button's resting label with its hidden hint, without the pending label. */
function label(b: HTMLElement): string {
  return (b.querySelector('.rl-button__label') ?? b).textContent!.trim()
}

/** One button per decision, in a group named Decisions; each says its effect. */
export const PostAndForm: Story = {
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    const group = canvas.getByRole('group', { name: 'Decisions' })
    await expect(within(group).getAllByRole('button').map(label)).toEqual([
      'Risk assessed (opens the form)',
      'Contained again (moves the session to Contained)',
      'Dismiss (moves the session to Dismissed)',
    ])
    await userEvent.click(canvas.getByRole('button', { name: /Contained again/ }))
    await expect(args.onDecide).toHaveBeenCalledWith('contained')
  },
}

/**
 * The pending button stays focusable as aria-disabled and keeps focus; the
 * others are disabled.
 */
export const Pending: Story = {
  args: { pending: new Set([decisionPendingKey('contained')]) },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    const pending = canvas.getByRole('button', { name: /Contained again/ })
    await expect(pending).toHaveAttribute('aria-disabled', 'true')
    await expect(pending).not.toBeDisabled()
    pending.focus()
    await userEvent.click(pending)
    await expect(pending).toHaveFocus()
    await expect(args.onDecide).not.toHaveBeenCalled()
    await expect(canvas.getByRole('button', { name: /Risk assessed/ })).toBeDisabled()
    await expect(canvas.getByRole('button', { name: /Dismiss/ })).toBeDisabled()
  },
}

/** The blocking key (a composer send) disables every button without marking one. */
export const Blocked: Story = {
  args: { pending: new Set([DECISIONS_BLOCKED_KEY]) },
  play: async ({ canvasElement }) => {
    for (const b of within(canvasElement).getAllByRole('button')) {
      await expect(b).toBeDisabled()
      await expect(b).not.toHaveAttribute('aria-busy')
    }
  },
}

/** A decision that ends the session as cancelled or failed is in the danger tone. */
export const Danger: Story = {
  args: { decisions: [breachDecisions[2]!] },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('button', { name: /Dismiss/ })).toHaveClass('rl-button--tone-danger')
  },
}

/** Many and long labels wrap in a narrow container; every button stays visible. */
export const ManyWrapping: Story = {
  args: {
    decisions: [
      ...breachDecisions,
      {
        key: 'a',
        messageType: 'a',
        label: 'Escalate to the data protection officer',
        toLabel: 'Escalated',
      },
      { key: 'b', messageType: 'b', label: 'Ask the processor for logs' },
      { key: 'c', messageType: 'c', label: 'リスク評価を依頼' },
    ],
  },
  render: (args) => ({
    components: { RlChannelDecisions },
    setup: () => ({ args }),
    template: '<div style="width:300px"><RlChannelDecisions v-bind="args" /></div>',
  }),
  play: async ({ canvasElement }) => {
    const group = within(canvasElement).getByRole('group')
    await expect(group.scrollWidth).toBeLessThanOrEqual(group.clientWidth + 1)
    for (const b of within(group).getAllByRole('button')) await expect(b).toBeVisible()
  },
}

/** The library texts in Dutch. */
export const Dutch: Story = {
  render: (args) => ({
    components: { RlChannelDecisions },
    setup() {
      provideMessages(NL_MESSAGES)
      return { args }
    },
    template: '<RlChannelDecisions v-bind="args" />',
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const group = canvas.getByRole('group', { name: 'Beslissingen' })
    await expect(within(group).getAllByRole('button').map(label)).toEqual([
      'Risk assessed (opent het formulier)',
      'Contained again (zet de sessie op Contained)',
      'Dismiss (zet de sessie op Dismissed)',
    ])
  },
}
