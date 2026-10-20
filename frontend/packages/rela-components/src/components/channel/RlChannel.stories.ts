import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, fn, userEvent, waitFor, within } from 'storybook/test'
import RlChannel from './RlChannel.vue'
import RlChannelComposer from './RlChannelComposer.vue'
import { lifecyclePendingKey } from './channelFields'
import {
  channelEnumOptions,
  channelEphemeral,
  channelLifecycle,
  channelMessages,
  channelMessageTypes,
  channelPresentation,
  manyChannelMessages,
  pickedPerson,
} from '../../fixtures/channels'

const meta: Meta<typeof RlChannel> = {
  title: 'Channel/Channel',
  component: RlChannel,
  args: {
    title: 'Incident room',
    state: 'open',
    messages: channelMessages,
    messageTypes: channelMessageTypes,
    postable: ['text', 'decision', 'finding'],
    lifecycle: channelLifecycle,
    allowedLifecycle: ['close'],
    ephemeral: channelEphemeral,
    presentation: channelPresentation,
    enumOptions: channelEnumOptions,
    entityHref: (r) => `#/entity/${r.id}`,
    onLifecycle: fn(),
    onReply: fn(),
    onJump: fn(),
    onLoadOlder: fn(),
  },
  render: (args) => ({
    components: { RlChannel, RlChannelComposer },
    setup: () => ({ args, pickRef: async () => pickedPerson }),
    template: `
      <div style="height:100vh">
        <RlChannel v-bind="args">
          <template #composer>
            <RlChannelComposer
              :message-types="args.messageTypes"
              :postable="args.postable"
              :presentation="args.presentation"
              :enum-options="args.enumOptions"
              :pick-ref="pickRef"
            />
          </template>
        </RlChannel>
      </div>
    `,
  }),
}
export default meta
type Story = StoryObj<typeof RlChannel>

/**
 * An open channel: readable, restricted, removed, pending and system messages,
 * an ephemeral status line, and the composer. Only the lifecycle operations
 * the user may run are shown.
 */
export const Open: Story = {
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByRole('button', { name: 'Close' })).toBeVisible()
    await expect(canvas.queryByRole('button', { name: 'Archive' })).toBeNull()
    await expect(canvas.queryByRole('button', { name: 'Delete channel' })).toBeNull()
    await expect(canvas.getByRole('log', { name: 'Messages' })).toBeInTheDocument()
    await expect(canvas.getByText(/reviewing access logs/)).toBeVisible()
    await expect(canvas.getByRole('form', { name: 'New message' })).toBeVisible()
    await userEvent.click(canvas.getByRole('button', { name: 'Close' }))
    await expect(args.onLifecycle).toHaveBeenCalledWith('close')
  },
}

/** A reply quote jumps to the original and focuses it when it is loaded. */
export const JumpToOriginal: Story = {
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('button', { name: /Reply to Triage agent/ }))
    await waitFor(() => expect(canvasElement.querySelector('[data-seq="2"]')).toHaveFocus())
    await expect(args.onJump).not.toHaveBeenCalled()
  },
}

/** A quote whose original is not loaded asks the app to fetch it. */
export const JumpToUnloaded: Story = {
  args: { messages: channelMessages.filter((m) => m.seq !== 2) },
  play: async ({ args, canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole('button', { name: /Reply to Triage agent/ }))
    await expect(args.onJump).toHaveBeenCalledWith(2)
  },
}

/**
 * A closed channel stays readable and says why it takes no messages. The
 * composer slot is not rendered, and with nothing postable no reply controls
 * appear either.
 */
export const Closed: Story = {
  args: { state: 'closed', allowedLifecycle: ['reopen', 'archive'], ephemeral: [] },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('Closed')).toBeVisible()
    await expect(canvas.getByText(/This channel is closed/)).toBeVisible()
    await expect(canvas.queryByRole('form')).toBeNull()
    await expect(canvas.queryByRole('button', { name: 'Reply' })).toBeNull()
    await expect(canvas.getByRole('button', { name: 'Reopen' })).toBeVisible()
  },
}

/** Archived reads the same way, with its own wording. */
export const Archived: Story = {
  args: { state: 'archived', allowedLifecycle: ['delete'], ephemeral: [] },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText(/This channel is archived/)).toBeVisible()
  },
}

/** A user who may only read: no composer, no lifecycle controls, no reply. */
export const ReadOnly: Story = {
  args: { postable: [], allowedLifecycle: [] },
  render: (args) => ({
    components: { RlChannel },
    setup: () => ({ args }),
    template: '<div style="height:100vh"><RlChannel v-bind="args" /></div>',
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.queryByRole('form')).toBeNull()
    await expect(canvas.queryByRole('button', { name: 'Close' })).toBeNull()
    await expect(canvas.queryByRole('button', { name: 'Reply' })).toBeNull()
  },
}

/** A lifecycle operation in flight. */
export const LifecyclePending: Story = {
  args: { pending: new Set([lifecyclePendingKey('close')]) },
}

/** No messages yet. */
export const Empty: Story = {
  args: { messages: [], ephemeral: [] },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('No messages yet.')).toBeVisible()
  },
}

/**
 * The most the app holds: 500 messages. The log opens at the newest, and
 * older ones load on request.
 */
export const FiveHundredMessages: Story = {
  args: { messages: manyChannelMessages(500), hasOlder: true, ephemeral: [] },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    const log = canvas.getByRole('log')
    await waitFor(() => expect(log.scrollTop).toBeGreaterThan(0))
    await expect(canvasElement.querySelectorAll('.rl-channel-message')).toHaveLength(500)
    await userEvent.click(canvas.getByRole('button', { name: 'Load older messages' }))
    await expect(args.onLoadOlder).toHaveBeenCalled()
  },
}
