import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, fn, userEvent, within } from 'storybook/test'
import RlChannelMessage from './RlChannelMessage.vue'
import { messagePendingKey } from './channelFields'
import type { ChannelMessageView } from './types'
import {
  breachDecisions,
  breachNotice,
  channelEnumOptions,
  channelMessages,
  channelMessageTypes,
  channelPresentation,
  questionChoices,
  questionMessage,
  questionMessageTypes,
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

/**
 * Mentions (TKT-6RXAC0). The app's renderer turns a code span the server
 * listed as a mention into an entity link for a person, and into a chip for
 * an agent or a login. A span the server did not list, here a person the
 * reader may not read, stays code. The HTML below is what the renderer gives.
 */
export const Mentions: Story = {
  args: {
    message: {
      seq: 22,
      rev: 1,
      type: 'text',
      author: { name: 'Ann' },
      at: '09:20',
      props: { body: 'usr-7 integration:triage-bot @jeroen USR-9' },
      html: {
        body:
          '<p><a href="#/entity/user/USR-7" class="entity-ref" title="USR-7">Pat Jansen</a>, ' +
          '<span class="mention">integration:triage-bot</span> and ' +
          '<span class="mention">@jeroen</span>: please check this. ' +
          '<code>USR-9</code> was not listed.</p>',
      },
      actions: [],
    },
    messageType: channelMessageTypes.find((t) => t.name === 'text'),
    canReply: false,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByRole('link', { name: 'Pat Jansen' })).toHaveAttribute('title', 'USR-7')
    const chips = [...canvasElement.querySelectorAll('.mention')].map((el) => el.textContent)
    await expect(chips).toEqual(['integration:triage-bot', '@jeroen'])
    await expect(canvasElement.querySelector('code')?.textContent).toBe('USR-9')
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

/**
 * A session notice: one line with the move and the time, and no controls,
 * not even reply.
 */
export const SessionNotice: Story = {
  args: {
    message: breachNotice(3, 'Detected', 'Contained'),
    messageType: undefined,
    canReply: true,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('Session moved from Detected to Contained')).toBeVisible()
    await expect(canvasElement.querySelector('time')).toHaveTextContent('09:12')
    await expect(canvas.queryAllByRole('button')).toEqual([])
  },
}

/** A move into the state the session was already in. */
export const SessionNoticeReentry: Story = {
  args: { message: breachNotice(3, 'Contained', 'Contained'), messageType: undefined },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('Session entered Contained again')).toBeVisible()
  },
}

/** A state the configuration dropped shows by its stored name. */
export const SessionNoticeRemovedState: Story = {
  args: { message: breachNotice(3, 'triage', 'Contained'), messageType: undefined },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('Session moved from triage to Contained')).toBeVisible()
  },
}

/**
 * The message that moved the session shows its decisions first among its
 * controls; the group is described by the message's type label and author.
 */
export const WithDecisions: Story = {
  args: { decisions: breachDecisions, onDecide: fn() },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    const group = canvas.getByRole('group', { name: 'Decisions' })
    // A card names its type label and author line; a prose message has no
    // type label, so only the author line.
    const described = (group.getAttribute('aria-describedby') ?? '').split(' ').filter(Boolean)
    await expect(described.length).toBeGreaterThan(0)
    for (const id of described) await expect(canvasElement.querySelector(`#${CSS.escape(id)}`)).not.toBeNull()
    const meta = canvasElement.querySelector('.rl-channel-message__meta')!
    await expect(described).toContain(meta.id)
    await userEvent.click(canvas.getByRole('button', { name: /Contained again/ }))
    await expect(args.onDecide).toHaveBeenCalledWith('contained')
  },
}

/**
 * A question with live choices (TKT-03FQPD): a group named Choices after
 * the fields, one button per option and Other. The options are not listed
 * a second time while the buttons show. A click emits `choose`.
 */
export const QuestionWithChoices: Story = {
  args: {
    message: questionMessage,
    messageType: questionMessageTypes[0],
    choices: questionChoices,
    onChoose: fn(),
  },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    const group = canvas.getByRole('group', { name: 'Choices' })
    const buttons = within(group).getAllByRole('button')
    await expect(buttons).toHaveLength(4)
    await expect(buttons[0]).toHaveAccessibleName('PostgreSQL (posts this answer as a reply)')
    await expect(buttons[3]).toHaveAccessibleName('Other answer… (opens the form)')
    // The long option shows in full.
    await expect(buttons[2]!.textContent).toContain('once the new views pass QA.')
    await expect(canvasElement.querySelector('.rl-channel-message__list')).toBeNull()
    await expect(canvas.queryByText('Options')).toBeNull()
    await userEvent.click(buttons[1]!)
    await expect(args.onChoose).toHaveBeenCalledWith('choice:12:1')
  },
}

/** The choices still show when the presentation hides the options field. */
export const ChoicesWithOptionsHidden: Story = {
  args: {
    ...QuestionWithChoices.args,
    presentation: { fields: [{ property: 'note' }, { property: 'options', hidden: true }] },
  },
  play: async ({ canvasElement }) => {
    const group = within(canvasElement).getByRole('group', { name: 'Choices' })
    await expect(within(group).getAllByRole('button')).toHaveLength(4)
  },
}

/**
 * An answered question, or a reader who may not answer: no buttons, and
 * the options render as a bulleted list of text, not as ref chips.
 */
export const QuestionAnswered: Story = {
  args: { message: questionMessage, messageType: questionMessageTypes[0] },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.queryByRole('group', { name: 'Choices' })).toBeNull()
    const items = [...canvasElement.querySelectorAll('.rl-channel-message__list li')].map((li) => li.textContent)
    await expect(items).toEqual(questionMessage.props!.options)
    await expect(canvasElement.querySelector('.rl-ref-chip')).toBeNull()
  },
}

/** A stored value that is not an array, for a list property, shows as text. */
export const ListValueNotArray: Story = {
  args: {
    message: { ...questionMessage, props: { note: 'Which?', options: 'PostgreSQL' } },
    messageType: questionMessageTypes[0],
  },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelector('.rl-channel-message__list')).toBeNull()
    await expect(within(canvasElement).getByText('PostgreSQL')).toBeVisible()
  },
}
