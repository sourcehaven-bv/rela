import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, fn, userEvent, within } from 'storybook/test'
import RlInbox from './RlInbox.vue'
import RlChannelDecisions from './RlChannelDecisions.vue'
import RlChannelMessage from './RlChannelMessage.vue'
import { DECISIONS_BLOCKED_KEY, decisionPendingKey } from './channelFields'
import type { ChannelDecisionView, InboxMentionRow, InboxUnreadRow, InboxWaitingRow } from './types'
import {
  breachDecisions,
  breachMessageTypes,
  channelMessages,
  questionChoices,
  questionMessage,
  questionMessageTypes,
} from '../../fixtures/channels'

// Sample rows shaped like the realtime mockup: an incident room, a design
// review and an approval. Kept inline so the stories depend on nothing else.
const link = (href: string) => ({ attrs: { href } })

const waiting: InboxWaitingRow[] = [
  {
    id: 'w1',
    anchorTitle: 'Checkout fails for EU cards',
    entityType: 'Incident',
    channel: 'Incident room',
    state: 'Waiting for the incident lead',
    since: '09:12',
    sinceIso: '2026-10-28T09:12:00Z',
    link: link('#/entity/incident/INC-12?channel=room'),
  },
  {
    id: 'w2',
    anchorTitle: 'Q4 pricing change',
    entityType: 'Decision',
    channel: 'Approvals',
    state: 'Awaiting approval',
    since: 'Yesterday',
    sinceIso: '2026-10-27T15:40:00Z',
    link: link('#/entity/decision/DEC-4?channel=approval'),
  },
]

const mentions: InboxMentionRow[] = [
  {
    id: 'm1',
    kind: 'mention',
    author: 'Bob',
    anchorTitle: 'New onboarding flow',
    channel: 'Design review',
    at: '10:04',
    atIso: '2026-10-28T10:04:00Z',
    link: link('#/entity/feature/FEAT-7?channel=review'),
  },
  {
    id: 'm2',
    kind: 'reply',
    author: 'Carol',
    anchorTitle: 'Checkout fails for EU cards',
    channel: 'Incident room',
    at: '09:30',
    atIso: '2026-10-28T09:30:00Z',
    link: link('#/entity/incident/INC-12?channel=room'),
  },
]

const unread: InboxUnreadRow[] = [
  {
    id: 'u1',
    anchorTitle: 'New onboarding flow',
    channel: 'Design review',
    unread: 4,
    link: link('#/entity/feature/FEAT-7?channel=review'),
  },
  {
    id: 'u2',
    anchorTitle: 'Release 3.2',
    channel: 'Release chat',
    unread: 120,
    link: link('#/entity/release/REL-32?channel=chat'),
  },
]

const meta: Meta<typeof RlInbox> = {
  title: 'Channel/Inbox',
  component: RlInbox,
  parameters: { layout: 'padded' },
  args: { waiting, mentions, unread, onMarkRead: fn() },
  render: (args) => ({
    components: { RlInbox },
    setup: () => ({ args }),
    template: '<div style="max-width:640px"><RlInbox v-bind="args" /></div>',
  }),
}
export default meta
type Story = StoryObj<typeof RlInbox>

/** Three sections; mention and unread rows offer mark read. */
export const Default: Story = {
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByRole('heading', { name: 'Waiting on you' })).toBeVisible()
    await expect(canvas.getByRole('heading', { name: 'Mentions and replies' })).toBeVisible()
    await expect(canvas.getByRole('heading', { name: 'Unread channels' })).toBeVisible()
    await expect(canvas.getByText('Bob mentioned you')).toBeVisible()
    await expect(canvas.getByText('Carol replied to you')).toBeVisible()
    await expect(canvas.getAllByRole('button', { name: 'Mark read' })).toHaveLength(4)
    await userEvent.click(canvas.getAllByRole('button', { name: 'Mark read' })[0]!)
    await expect(args.onMarkRead).toHaveBeenCalledWith('m1')
  },
}

/** Nothing waits, nothing is unread: three empty states. */
export const Empty: Story = {
  args: { waiting: [], mentions: [], unread: [] },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('Nothing waits on you.')).toBeVisible()
    await expect(canvas.getByText('No unread mentions or replies.')).toBeVisible()
    await expect(canvas.getByText('All channels are read.')).toBeVisible()
  },
}

/** Only unread chat: the sidebar shows a dot, the page one section of rows. */
export const OnlyUnread: Story = {
  args: { waiting: [], mentions: [] },
}

/** Some channel settings failed to load; the list may be incomplete. */
export const Degraded: Story = {
  args: { degraded: true },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText(/could not be loaded/)).toBeVisible()
  },
}

/** Long titles are cut with an ellipsis rather than wrapping the row. */
export const LongTitles: Story = {
  args: {
    waiting: [
      {
        ...waiting[0]!,
        anchorTitle:
          'Checkout fails for EU cards issued by banks that require a second factor on every payment above fifty euros',
        state: 'Waiting for the incident lead or the on-call payments engineer',
      },
    ],
    mentions: [
      {
        ...mentions[0]!,
        author: 'Bartholomew van der Heijden-Oosterhout',
        anchorTitle: 'A very long feature title that keeps going well past the width of the page',
      },
    ],
  },
}

/** More waiting sessions than the first page holds: the note names the total. */
export const Many: Story = {
  args: {
    waiting: Array.from({ length: 50 }, (_, i) => ({
      ...waiting[i % 2]!,
      id: `w${i}`,
      anchorTitle: `${waiting[i % 2]!.anchorTitle} #${i + 1}`,
    })),
    waitingMore: { shown: 50, total: '20000+' },
  },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('Showing 50 of 20000+')).toBeVisible()
  },
}

/*
 * The waiting-detail slot (TKT-V05706X): the app shows the message that
 * moved the session, with its choices and decisions, below the row's link.
 */

interface Detail {
  message?: typeof questionMessage
  messageType?: (typeof questionMessageTypes)[number]
  choices?: ChannelDecisionView[]
  decisions?: ChannelDecisionView[]
  pending?: ReadonlySet<string>
}

function withDetail(details: Record<string, Detail>): Story['render'] {
  return (args) => ({
    components: { RlInbox, RlChannelMessage, RlChannelDecisions },
    setup: () => ({ args, details }),
    template: `<div style="max-width:640px"><RlInbox v-bind="args">
      <template #waiting-detail="{ row }">
        <template v-if="details[row.id]">
          <RlChannelMessage
            v-if="details[row.id].message"
            :message="details[row.id].message"
            :message-type="details[row.id].messageType"
            :can-reply="false"
            :choices="details[row.id].choices"
            :decisions="details[row.id].decisions"
            :pending="details[row.id].pending"
            @choose="args.onChoose"
            @decide="args.onDecide"
          />
          <RlChannelDecisions
            v-else
            :decisions="details[row.id].decisions"
            :pending="details[row.id].pending"
            @decide="args.onDecide"
          />
        </template>
      </template>
    </RlInbox></div>`,
  })
}

const breachCause = channelMessages[channelMessages.length - 1]!

/** A question with choices: one click on an option answers it from the inbox. */
export const QuestionWithChoices: Story = {
  args: { waiting: [waiting[0]!], mentions: [], unread: [], onChoose: fn() } as never,
  render: withDetail({
    w1: {
      message: questionMessage,
      messageType: questionMessageTypes[0],
      choices: questionChoices,
    },
  }),
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    const group = canvas.getByRole('group', { name: 'Choices' })
    await expect(within(group).getAllByRole('button')).toHaveLength(4)
    await userEvent.click(within(group).getByRole('button', { name: /SQLite/ }))
    await expect(
      (args as unknown as { onChoose: ReturnType<typeof fn> }).onChoose
    ).toHaveBeenCalledWith('choice:12:1')
  },
}

/** A plan to approve: the session's decisions show on the message that moved it. */
export const PlanApproval: Story = {
  args: { waiting: [waiting[1]!], mentions: [], unread: [], onDecide: fn() } as never,
  render: withDetail({
    w2: {
      message: breachCause,
      messageType: breachMessageTypes.find((t) => t.name === breachCause.type),
      decisions: breachDecisions,
    },
  }),
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('button', { name: /Contained again/ }))
    await expect(
      (args as unknown as { onDecide: ReturnType<typeof fn> }).onDecide
    ).toHaveBeenCalledWith('contained')
  },
}

/** A row whose waiting message is not known: only the decisions show. */
export const WithoutMessage: Story = {
  args: { waiting: [waiting[1]!], mentions: [], unread: [], onDecide: fn() } as never,
  render: withDetail({ w2: { decisions: breachDecisions } }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByRole('group', { name: 'Decisions' })).toBeVisible()
    await expect(canvas.getByRole('link')).toBeVisible()
  },
}

/** A click in flight: its button works, and every other decision waits. */
export const PendingClick: Story = {
  args: { waiting: [waiting[1]!], mentions: [], unread: [], onDecide: fn() } as never,
  render: withDetail({
    w2: {
      decisions: breachDecisions,
      pending: new Set([decisionPendingKey('contained'), DECISIONS_BLOCKED_KEY]),
    },
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByRole('button', { name: /Dismiss/ })).toBeDisabled()
  },
}
