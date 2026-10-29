import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import { expect, fn, userEvent, waitFor, within } from 'storybook/test'
import RlChannel from './RlChannel.vue'
import RlChannelComposer from './RlChannelComposer.vue'
import RlChannelSession from './RlChannelSession.vue'
import { lifecyclePendingKey } from './channelFields'
import { provideMessages } from '../../composables/useMessages'
import type { ChannelMessageView } from './types'
import {
  breachDecisions,
  breachMessages,
  breachMessageTypes,
  breachNotice,
  breachSessionDetected,
  breachTransitions,
  channelEnumOptions,
  channelEphemeral,
  channelLifecycle,
  channelMessages,
  channelMessageTypes,
  channelPresentation,
  manyChannelMessages,
  pickedPerson,
  answerMessage,
  questionChoices,
  questionMessage,
  questionMessageTypes,
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

/**
 * A harness around a channel whose messages change: buttons append a new
 * message, prepend a page of older ones, trim the oldest (the app's cap) and
 * show a channel that mounted hidden.
 */
const liveRender: Story['render'] = (args) => ({
  components: { RlChannel },
  setup() {
    const messages = ref<ChannelMessageView[]>(manyChannelMessages(60, 41))
    const shown = ref(true)
    const append = () => {
      const last = messages.value[messages.value.length - 1]!.seq
      messages.value = [...messages.value, ...manyChannelMessages(1, last + 1)]
    }
    const prepend = () => {
      const first = messages.value[0]!.seq
      messages.value = [...manyChannelMessages(20, first - 20), ...messages.value]
    }
    const trim = () => {
      messages.value = messages.value.slice(10)
    }
    const hideThenShow = async () => {
      shown.value = false
      messages.value = manyChannelMessages(60, 41)
      await new Promise((r) => setTimeout(r, 20))
      shown.value = true
    }
    return { args, messages, shown, append, prepend, trim, hideThenShow }
  },
  template: `
    <div>
      <button type="button" data-testid="append" @click="append">Append</button>
      <button type="button" data-testid="prepend" @click="prepend">Prepend</button>
      <button type="button" data-testid="trim" @click="trim">Trim</button>
      <button type="button" data-testid="hide" @click="hideThenShow">Hide then show</button>
      <div v-show="shown" style="height:400px">
        <RlChannel v-bind="args" :messages="messages" />
      </div>
    </div>`,
})

/** Offset of a message from the top of the log, in pixels. */
function offsetOf(log: HTMLElement, seq: number): number {
  const row = log.querySelector<HTMLElement>(`[data-seq="${seq}"]`)!
  return row.getBoundingClientRect().top - log.getBoundingClientRect().top
}

function isAtBottom(log: HTMLElement): boolean {
  return log.scrollHeight - log.scrollTop - log.clientHeight < 2
}

/**
 * A reader at the bottom follows a new message, and hears it announced; a
 * reader who scrolled up stays where they are.
 */
export const FollowAtBottom: Story = {
  args: { ephemeral: [], hasOlder: true },
  render: liveRender,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const log = canvas.getByRole('log')
    await waitFor(() => expect(isAtBottom(log)).toBe(true))
    await userEvent.click(canvas.getByTestId('append'))
    await waitFor(() => expect(canvasElement.querySelector('[data-seq="101"]')).not.toBeNull())
    await waitFor(() => expect(isAtBottom(log)).toBe(true))
    await expect(canvasElement.querySelector('[aria-live="polite"][aria-atomic="true"]')).toHaveTextContent(
      'Message from Joost Bakker',
    )

    log.scrollTop = 200
    log.dispatchEvent(new Event('scroll'))
    await waitFor(() => expect(log.scrollTop).toBe(200))
    const before = offsetOf(log, 50)
    await userEvent.click(canvas.getByTestId('append'))
    await waitFor(() => expect(canvasElement.querySelector('[data-seq="102"]')).not.toBeNull())
    await expect(isAtBottom(log)).toBe(false)
    await expect(Math.abs(offsetOf(log, 50) - before)).toBeLessThan(2)
  },
}

/**
 * Loading older messages above, or the app trimming the oldest at its cap,
 * keeps the first visible message where the reader had it.
 */
export const PreserveOnPrependAndTrim: Story = {
  args: { ephemeral: [], hasOlder: true },
  render: liveRender,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const log = canvas.getByRole('log')
    await waitFor(() => expect(isAtBottom(log)).toBe(true))
    log.scrollTop = 300
    log.dispatchEvent(new Event('scroll'))
    const anchor = 60
    const before = offsetOf(log, anchor)

    await userEvent.click(canvas.getByTestId('prepend'))
    await waitFor(() => expect(canvasElement.querySelector('[data-seq="21"]')).not.toBeNull())
    await waitFor(() => expect(Math.abs(offsetOf(log, anchor) - before)).toBeLessThan(2))

    await userEvent.click(canvas.getByTestId('trim'))
    await waitFor(() => expect(canvasElement.querySelector('[data-seq="21"]')).toBeNull())
    await waitFor(() => expect(Math.abs(offsetOf(log, anchor) - before)).toBeLessThan(2))
  },
}

/** A log first shown after it mounted (a hidden tab) still opens at the newest message. */
export const OpensAtBottomWhenShown: Story = {
  args: { ephemeral: [] },
  render: liveRender,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const log = canvas.getByRole('log')
    await userEvent.click(canvas.getByTestId('hide'))
    await waitFor(() => expect(log.clientHeight).toBeGreaterThan(0))
    await waitFor(() => expect(isAtBottom(log)).toBe(true))
  },
}

const typists = ['Ann', 'Bo', 'Cy', 'Dee', 'Eve'].map((name) => ({ key: name, author: { name } }))

/**
 * Who else is typing: one line in its own polite, atomic live region. It
 * names at most two typists and counts the rest. A new head with the same
 * typists leaves the line as it was, so it is not read out again.
 */
export const Typing: Story = {
  args: { ephemeral: [] },
  render: (args) => ({
    components: { RlChannel },
    setup: () => {
      const typing = ref(typists.slice(0, 1))
      const show = (n: number) => (typing.value = typists.slice(0, n).map((t) => ({ ...t })))
      return { args, typing, show }
    },
    template: `
      <div style="height:100vh">
        <button v-for="n in [0, 1, 2, 3, 5]" :key="n" :data-typists="n" @click="show(n)">{{ n }}</button>
        <RlChannel v-bind="args" :typing="typing" typing-offered>
          <template #composer><div>composer</div></template>
        </RlChannel>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const line = canvasElement.querySelector('.rl-channel__typing') as HTMLElement
    await expect(line).toHaveAttribute('aria-live', 'polite')
    await expect(line).toHaveAttribute('aria-atomic', 'true')
    const show = (n: number) =>
      userEvent.click(canvasElement.querySelector(`[data-typists="${n}"]`) as HTMLElement)
    for (const [n, text] of [
      [1, 'Ann is typing…'],
      [2, 'Ann and Bo are typing…'],
      [3, 'Ann, Bo and 1 other are typing…'],
      [5, 'Ann, Bo and 3 others are typing…'],
    ] as const) {
      await show(n)
      await waitFor(() => expect(line).toHaveTextContent(text))
    }
    const node = line.firstChild
    await show(5)
    await expect(line.firstChild).toBe(node)
    await expect(line).toHaveTextContent('Ann, Bo and 3 others are typing…')
    await show(0)
    await waitFor(() => expect(line).toHaveTextContent(/^$/))
  },
}

/**
 * No typing line where the type does not offer typing, or where there is
 * no composer: no blank row and no extra live region.
 */
export const TypingLineOnlyWhereOffered: Story = {
  args: { ephemeral: [] },
  render: (args) => ({
    components: { RlChannel },
    setup: () => ({ args, typing: typists.slice(0, 2) }),
    template: `
      <div>
        <div data-case="not offered">
          <RlChannel v-bind="args" :typing="typing">
            <template #composer><div>composer</div></template>
          </RlChannel>
        </div>
        <div data-case="no composer">
          <RlChannel v-bind="args" :typing="typing" typing-offered />
        </div>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    for (const c of ['not offered', 'no composer']) {
      const root = canvasElement.querySelector(`[data-case="${c}"]`) as HTMLElement
      await expect(root.querySelector('.rl-channel__typing')).toBeNull()
    }
  },
}

/* Sessions --------------------------------------------------------------- */

/**
 * The breach room: the session stepper between the header and the log,
 * notices in the log, and the composer with the transition types first.
 */
export const SessionRoom: Story = {
  args: {
    title: 'Breach response',
    messages: breachMessages,
    messageTypes: breachMessageTypes,
    postable: ['text', 'contained', 'dismiss'],
    ephemeral: [],
    presentation: undefined,
  },
  render: (args) => ({
    components: { RlChannel, RlChannelComposer, RlChannelSession },
    setup: () => ({ args, session: breachSessionDetected, transitions: breachTransitions }),
    template: `
      <div style="height:100vh">
        <RlChannel v-bind="args">
          <template #session>
            <RlChannelSession :session="session" />
          </template>
          <template #composer>
            <RlChannelComposer
              :message-types="args.messageTypes"
              :postable="args.postable"
              :transitions="transitions"
            />
          </template>
        </RlChannel>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const header = canvasElement.querySelector('header')!
    const stepper = canvasElement.querySelector('.rl-channel-session')!
    const log = canvas.getByRole('log')
    await expect(header.compareDocumentPosition(stepper) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    await expect(stepper.compareDocumentPosition(log) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    await expect(within(log).getByText('Session moved from Detected to Contained')).toBeVisible()
    const group = canvas.getByRole('radiogroup', { name: 'Message type' })
    await expect(within(group).getAllByRole('radio')[0]).toHaveTextContent('Contained')
  },
}

/** The session slot renders on a closed channel too. */
export const SessionRoomClosed: Story = {
  ...SessionRoom,
  args: { ...SessionRoom.args, state: 'closed' },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelector('.rl-channel-session')).not.toBeNull()
  },
}

const noticeRender: Story['render'] = (args) => ({
  components: { RlChannel },
  setup() {
    const messages = ref<ChannelMessageView[]>(breachMessages.slice(0, 1))
    const notice = () => {
      messages.value = [...messages.value, breachNotice(2, 'Detected', 'Contained')]
    }
    const both = () => {
      messages.value = [
        ...messages.value,
        { ...breachMessages[1]!, seq: 2, rev: 2 },
        breachNotice(3, 'Detected', 'Contained'),
      ]
    }
    return { args, messages, notice, both }
  },
  template: `
    <div>
      <button type="button" data-testid="notice" @click="notice">Notice</button>
      <button type="button" data-testid="both" @click="both">Both</button>
      <div style="height:400px">
        <RlChannel v-bind="args" :messages="messages" />
      </div>
    </div>`,
})

function announced(canvasElement: HTMLElement): Element {
  return canvasElement.querySelector('[aria-live="polite"][aria-atomic="true"]')!
}

/** A notice that arrives alone is announced by its text. */
export const AnnouncesNotice: Story = {
  args: { messageTypes: breachMessageTypes, ephemeral: [], presentation: undefined },
  render: noticeRender,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByTestId('notice'))
    await waitFor(() =>
      expect(announced(canvasElement)).toHaveTextContent(/^Session moved from Detected to Contained\.$/),
    )
  },
}

/** A message and its notice in one update: the message, then the move. */
export const AnnouncesMessageAndNotice: Story = {
  args: { messageTypes: breachMessageTypes, ephemeral: [], presentation: undefined },
  render: noticeRender,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByTestId('both'))
    await waitFor(() =>
      expect(announced(canvasElement)).toHaveTextContent(
        /^Contained from Ann\. Session moved from Detected to Contained\.$/,
      ),
    )
  },
}

const overrideRender: Story['render'] = (args, ctx) => {
  const inner = noticeRender(args, ctx) as { setup: () => unknown }
  return {
    ...inner,
    setup() {
      provideMessages({
        channelNewMessages: ({ count }) => `${count} nieuw`,
        channelNewNotices: ({ notices, messages }) => [messages, ...notices].filter(Boolean).join(' / '),
      })
      return inner.setup()
    },
  }
}

/**
 * An app that overrides channelNewMessages under its contract (a count of
 * at least 1, no notices) is not asked about a lone notice:
 * channelNewNotices announces it.
 */
export const AnnouncesNoticeWithOverride: Story = {
  args: { messageTypes: breachMessageTypes, ephemeral: [], presentation: undefined },
  render: overrideRender,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByTestId('notice'))
    await waitFor(() =>
      expect(announced(canvasElement)).toHaveTextContent(/^Session moved from Detected to Contained$/),
    )
  },
}

/** channelNewNotices receives the channelNewMessages text of the other messages. */
export const AnnouncesMessageAndNoticeWithOverride: Story = {
  args: { messageTypes: breachMessageTypes, ephemeral: [], presentation: undefined },
  render: overrideRender,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByTestId('both'))
    await waitFor(() =>
      expect(announced(canvasElement)).toHaveTextContent(/^1 nieuw \/ Session moved from Detected to Contained$/),
    )
  },
}

/** The decisions show on the message that moved the session, and on no other row. */
export const DecisionsOnMessage: Story = {
  args: {
    messages: breachMessages,
    messageTypes: breachMessageTypes,
    postable: ['text', 'contained', 'dismiss'],
    decisions: breachDecisions,
    decisionSeq: 2,
    onDecide: fn(),
  },
  play: async ({ args, canvasElement }) => {
    const groups = canvasElement.querySelectorAll('.rl-channel-message [role="group"].rl-channel-decisions')
    await expect(groups).toHaveLength(1)
    const row = groups[0]!.closest('.rl-channel-message')!
    await expect(row).toHaveAttribute('data-seq', '2')
    await userEvent.click(within(row as HTMLElement).getByRole('button', { name: /Contained again/ }))
    await expect(args.onDecide).toHaveBeenCalledWith('contained', 'row')
  },
}

/**
 * A question with live choices on its own row (TKT-03FQPD); the answered
 * question above it shows its options as a list. A click emits `choose`.
 */
export const WithChoices: Story = {
  args: {
    title: 'Importer run',
    messages: [
      { ...questionMessage, seq: 10, rev: 10 },
      { ...answerMessage, seq: 11, rev: 11, replyTo: { ...answerMessage.replyTo!, seq: 10 } },
      questionMessage,
    ],
    messageTypes: questionMessageTypes,
    postable: ['answer'],
    choices: new Map([[12, questionChoices]]),
    onChoose: fn(),
  },
  play: async ({ args, canvasElement }) => {
    const groups = canvasElement.querySelectorAll('[role="group"][aria-label="Choices"]')
    await expect(groups).toHaveLength(1)
    const row = groups[0]!.closest('.rl-channel-message')!
    await expect(row).toHaveAttribute('data-seq', '12')
    const answered = canvasElement.querySelector('.rl-channel-message[data-seq="10"]')!
    await expect(answered.querySelectorAll('.rl-channel-message__list li')).toHaveLength(3)
    await userEvent.click(within(row as HTMLElement).getByRole('button', { name: /^SQLite/ }))
    await expect(args.onChoose).toHaveBeenCalledWith('choice:12:1')
  },
}
