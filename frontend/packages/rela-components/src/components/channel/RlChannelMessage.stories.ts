import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, fn, userEvent, within } from 'storybook/test'
import RlChannelMessage from './RlChannelMessage.vue'
import { messagePendingKey } from './channelFields'
import type { ChannelMessageView } from './types'
import {
  channelEnumOptions,
  channelMessages,
  channelMessageTypes,
  channelPresentation,
} from '../../fixtures/channels'

function msg(seq: number): ChannelMessageView {
  return channelMessages.find((m) => m.seq === seq)!
}

function typeOf(m: ChannelMessageView) {
  return channelMessageTypes.find((t) => t.name === m.type)
}

const meta: Meta<typeof RlChannelMessage> = {
  title: 'Channel/Channel message',
  component: RlChannelMessage,
  parameters: { layout: 'padded' },
  args: {
    message: msg(3),
    messageType: typeOf(msg(3)),
    enumOptions: channelEnumOptions,
    entityHref: (r) => `#/entity/${r.id}`,
    canReply: true,
    onReply: fn(),
    onEdit: fn(),
    onDelete: fn(),
    onRemove: fn(),
    onRelease: fn(),
    onAction: fn(),
    onJump: fn(),
  },
}
export default meta
type Story = StoryObj<typeof RlChannelMessage>

/**
 * The user's own message, with the edit and delete rights the server offered,
 * and a reply quote that jumps to the original.
 */
export const OwnMessage: Story = {
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('(edited)')).toHaveAttribute('title', 'Edited at 09:12')
    await expect(canvas.queryByRole('button', { name: 'Remove…' })).toBeNull()
    await userEvent.click(canvas.getByRole('button', { name: 'Edit' }))
    await expect(args.onEdit).toHaveBeenCalledWith(3)
    await userEvent.click(canvas.getByRole('button', { name: 'Delete' }))
    await expect(args.onDelete).toHaveBeenCalledWith(3)
    await userEvent.click(canvas.getByRole('button', { name: /Reply to Triage agent/ }))
    await expect(args.onJump).toHaveBeenCalledWith(2)
  },
}

/**
 * A moderator sees Remove, not Delete: they are separate rights, and each
 * control appears only when the message's `actions` names it.
 */
export const Moderator: Story = {
  args: { message: msg(5), messageType: typeOf(msg(5)), presentation: channelPresentation.messages!.decision },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.queryByRole('button', { name: 'Delete' })).toBeNull()
    await expect(canvas.queryByRole('button', { name: 'Edit' })).toBeNull()
    await userEvent.click(canvas.getByRole('button', { name: 'Remove…' }))
    await expect(args.onRemove).toHaveBeenCalledWith(5)
    // The presentation orders and labels the fields, and hides internal_note.
    const terms = [...canvasElement.querySelectorAll('dt')].map((d) => d.textContent)
    await expect(terms).toEqual(['Decision', 'Severity', 'Owner', 'Notify the DPA', 'due'])
    await expect(canvas.queryByText(/Not shown/)).toBeNull()
    await expect(canvas.getByText('High')).toBeVisible()
    await expect(canvas.getByText('14 Oct 2026')).toBeVisible()
    await expect(canvas.getByText('Restricted reference')).toBeVisible()
  },
}

/** No rights at all: no controls, not disabled ones. */
export const ReadOnly: Story = {
  args: { message: { ...msg(3), actions: [] }, canReply: false },
  play: async ({ canvasElement }) => {
    const buttons = within(canvasElement).queryAllByRole('button')
    // Only the reply quote is a button.
    await expect(buttons.map((b) => b.textContent?.trim())).toEqual([
      expect.stringContaining('Reply to Triage agent'),
    ])
  },
}

/** A typed message with actions: promote uses the primary button. */
export const WithActions: Story = {
  args: { message: msg(2), messageType: typeOf(msg(2)) },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByRole('link', { name: /SYS-12/ })).toHaveAttribute('href', '#/entity/SYS-12')
    await userEvent.click(canvas.getByRole('button', { name: 'Promote to risk' }))
    await expect(args.onAction).toHaveBeenCalledWith(2, 'promote')
  },
}

/** While an action runs its button swaps to the pending label. */
export const ActionPending: Story = {
  args: {
    message: msg(2),
    messageType: typeOf(msg(2)),
    pending: new Set([messagePendingKey(2, 'promote')]),
  },
}

/** A promote waiting to complete, with Release for a user allowed to release it. */
export const PendingRelease: Story = {
  args: { message: msg(6), messageType: typeOf(msg(6)) },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('Promote to risk: pending')).toBeVisible()
    await expect(canvas.queryByRole('button', { name: 'Promote to risk' })).toBeNull()
    await userEvent.click(canvas.getByRole('button', { name: 'Release' }))
    await expect(args.onRelease).toHaveBeenCalledWith(6)
  },
}

/** A completed promote links to the entity it created. */
export const Promoted: Story = {
  args: { message: msg(7), messageType: typeOf(msg(7)) },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('Promote to risk: done')).toBeVisible()
    await expect(canvas.getByRole('link', { name: /RISK-88/ })).toBeVisible()
  },
}

/**
 * A removed message shows a placeholder and none of what it said. A remover
 * also sees who removed it and why, because the server sends those to them.
 */
export const Removed: Story = {
  args: { message: msg(4), messageType: undefined },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(
      canvas.getByText('Removed by a moderator (Joost Bakker): Posted customer data in the clear'),
    ).toBeVisible()
    await expect(canvas.queryAllByRole('button')).toHaveLength(0)
  },
}

/** The same removal, for a reader the server tells less. */
export const RemovedByAuthor: Story = {
  args: { message: { seq: 9, rev: 3, removed: { kind: 'author' } }, messageType: undefined },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('Deleted by its author')).toBeVisible()
  },
}

/** A system post shows the system badge. */
export const SystemPost: Story = {
  args: { message: msg(1), messageType: typeOf(msg(1)), canReply: false },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('system')).toBeVisible()
  },
}

/**
 * Strings are text wherever they come from: a property value, an author name,
 * a removal reason. Markup in them creates no element.
 */
export const MarkupIsText: Story = {
  args: {
    message: {
      seq: 20,
      rev: 1,
      type: 'finding',
      author: { name: '<b>Mallory</b>' },
      props: { title: '<img src=x onerror="alert(1)">', detail: '<script>alert(1)</script>' },
      actions: [],
    },
    messageType: channelMessageTypes.find((t) => t.name === 'finding'),
    canReply: false,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvasElement.querySelector('img, script, b')).toBeNull()
    await expect(canvas.getByText('<img src=x onerror="alert(1)">')).toBeVisible()
    // A markdown property without app-rendered HTML is shown as plain text.
    await expect(canvas.getByText('<script>alert(1)</script>')).toBeVisible()
    await expect(canvas.getByText('<b>Mallory</b>')).toBeVisible()
  },
}

/** A removal reason with markup is text too. */
export const MarkupInReason: Story = {
  args: {
    message: { seq: 21, rev: 2, removed: { kind: 'moderator', reason: '<img src=x onerror="alert(1)">' } },
    messageType: undefined,
  },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelector('img')).toBeNull()
  },
}
