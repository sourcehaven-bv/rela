import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { nextTick, ref } from 'vue'
import { expect, fn, userEvent, waitFor, within } from 'storybook/test'
import RlChannelComposer from './RlChannelComposer.vue'
import type {
  ChannelEditEvent,
  ChannelEditing,
  ChannelMessageTypeDef,
  ChannelPostEvent,
  ChannelReadableRef,
} from './types'
import {
  breachMessageTypes,
  breachTransitions,
  channelEnumOptions,
  channelMessageTypes,
  channelPresentation,
  pickedPerson,
} from '../../fixtures/channels'

const meta: Meta<typeof RlChannelComposer> = {
  title: 'Channel/Channel composer',
  component: RlChannelComposer,
  parameters: { layout: 'padded' },
  args: {
    messageTypes: channelMessageTypes,
    postable: ['text', 'decision', 'finding', 'status'],
    presentation: channelPresentation,
    enumOptions: channelEnumOptions,
    pickRef: async () => pickedPerson,
    timeZoneLabel: 'Europe/Amsterdam',
    onPost: fn(),
    onSave: fn(),
    onCancelReply: fn(),
    onCancelEdit: fn(),
    onSelect: fn(),
  },
  render: (args) => ({
    components: { RlChannelComposer },
    setup: () => ({ args }),
    template: '<div style="max-width:640px"><RlChannelComposer v-bind="args" /></div>',
  }),
}
export default meta
type Story = StoryObj<typeof RlChannelComposer>

/** A plain message: one markdown field, sent with Ctrl+Enter or the button. */
export const Playground: Story = {
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(canvas.getByRole('textbox', { name: /Message/ }), 'Logs checked.')
    await userEvent.keyboard('{Control>}{Enter}{/Control}')
    await expect(args.onPost).toHaveBeenCalledWith({
      type: 'text',
      values: { body: 'Logs checked.' },
      rendered: ['body'],
      replyTo: undefined,
    })
  },
}

/** Only the types the user may post now are offered; with one, no selector. */
export const OnlyPostableTypes: Story = {
  args: { postable: ['text', 'finding'] },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const group = canvas.getByRole('radiogroup', { name: 'Message type' })
    const options = within(group).getAllByRole('radio').map((r) => r.textContent?.trim())
    await expect(options).toEqual(['Message', 'Finding'])
  },
}

/** One postable type: no type selector at all. */
export const SingleType: Story = {
  args: { postable: ['text'] },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).queryByRole('radiogroup')).toBeNull()
  },
}

/** Nothing postable: the composer renders nothing, and the caller says why. */
export const NothingPostable: Story = {
  args: { postable: [] },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelector('form')).toBeNull()
  },
}

/**
 * The decision type follows its presentation: the listed fields in order with
 * their labels, help and widgets, the hidden one left out, then the unlisted
 * ones required first and by name.
 */
export const Presentation: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('radio', { name: 'Decision' }))
    const labels = [...canvasElement.querySelectorAll('.rl-channel-composer__field')].map((f) =>
      f.querySelector('.rl-field__label, .rl-field__wrapped-label')?.textContent?.replaceAll('*', '').trim(),
    )
    await expect(labels).toEqual(['Decision', 'Severity', 'Owner', 'Notify the DPA', 'due'])
    await expect(canvas.getByText('One sentence.')).toBeVisible()
    await expect(canvas.getByRole('group', { name: 'Owner' })).toBeVisible()
    await expect(canvas.queryByText('internal_note')).toBeNull()
    await expect(canvasElement.querySelectorAll('.rl-channel-composer__field')).toHaveLength(5)
  },
}

/**
 * A required field blocks the send and says so; `max` counts code points, as
 * the server does, so four emoji are four characters.
 */
export const RequiredAndMax: Story = {
  args: {
    messageTypes: [
      {
        name: 'note',
        label: 'Note',
        persist: 'log',
        reply: false,
        properties: [
          { name: 'title', label: 'Title', type: 'string', required: true, max: 4 },
          { name: 'extra', label: 'Extra', type: 'string' },
        ],
        actions: [],
      },
    ],
    postable: ['note'],
    presentation: undefined,
  },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('button', { name: 'Send' }))
    await expect(args.onPost).not.toHaveBeenCalled()
    await expect(canvas.getByText('Required.')).toBeVisible()
    const title = canvas.getByRole('textbox', { name: /Title/ })
    await waitFor(() => expect(title).toHaveFocus())
    await userEvent.type(title, '🔥🔥🔥🔥🔥')
    await userEvent.click(canvas.getByRole('button', { name: 'Send' }))
    await expect(canvas.getByText('At most 4 characters.')).toBeVisible()
    await userEvent.clear(title)
    await userEvent.type(title, '🔥🔥🔥🔥')
    await userEvent.click(canvas.getByRole('button', { name: 'Send' }))
    // An empty optional field is sent as absent.
    await expect(args.onPost).toHaveBeenCalledWith({
      type: 'note',
      values: { title: '🔥🔥🔥🔥', extra: undefined },
      rendered: ['title', 'extra'],
      replyTo: undefined,
    })
  },
}

/** Every widget, with the values sent raw for the app to convert. */
export const EveryWidget: Story = {
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('radio', { name: 'Decision' }))
    await userEvent.type(canvas.getByRole('textbox', { name: /Decision/ }), 'Notify the DPA.')
    await userEvent.selectOptions(canvas.getByRole('combobox', { name: /Severity/ }), 'high')
    await userEvent.click(canvas.getByRole('button', { name: 'Choose a person…' }))
    await waitFor(() => expect(canvas.getByText('PER-7')).toBeVisible())
    await userEvent.click(canvas.getByRole('checkbox', { name: /Notify the DPA/ }))
    await userEvent.click(canvas.getByRole('button', { name: 'Send' }))
    await expect(args.onPost).toHaveBeenCalledWith({
      type: 'decision',
      values: { summary: 'Notify the DPA.', severity: 'high', owner: 'PER-7', notify_dpa: true, due: undefined },
      rendered: ['summary', 'severity', 'owner', 'notify_dpa', 'due'],
      replyTo: undefined,
    })
  },
}

/** Field messages from the app (a server 422) and a general banner. */
export const ServerErrors: Story = {
  args: {
    postable: ['finding'],
    errors: { affected: 'must be a whole number' },
    error: 'The server refused the message: too many properties.',
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('must be a whole number')).toBeVisible()
    await expect(canvas.getByText(/too many properties/)).toBeVisible()
  },
}

/** While a reply is written only the types that may reply are offered. */
export const Reply: Story = {
  args: { replyTo: { seq: 2, author: 'Triage agent', excerpt: 'Bucket policy allows public read' } },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('Replying to Triage agent')).toBeVisible()
    await expect(canvas.queryByRole('radio', { name: 'Decision' })).toBeNull()
    await userEvent.type(canvas.getByRole('textbox', { name: /Message/ }), 'On it.')
    await userEvent.click(canvas.getByRole('button', { name: 'Send' }))
    await expect(args.onPost).toHaveBeenCalledWith(expect.objectContaining({ replyTo: 2 }))
    await userEvent.click(canvas.getByRole('button', { name: 'Cancel reply' }))
    await expect(args.onCancelReply).toHaveBeenCalled()
  },
}

/** Edit mode: the type is fixed, the fields are prefilled, and Save emits the seq. */
export const Edit: Story = {
  args: {
    editing: {
      seq: 5,
      type: 'decision',
      values: { summary: 'Notify the DPA within 72 hours.', severity: 'high', notify_dpa: true, owner: 'PER-7' },
      refs: { owner: pickedPerson },
    },
  },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.queryByRole('radiogroup')).toBeNull()
    const summary = canvas.getByRole('textbox', { name: /Decision/ })
    await expect(summary).toHaveValue('Notify the DPA within 72 hours.')
    await userEvent.type(summary, ' Done.')
    await userEvent.click(canvas.getByRole('button', { name: 'Save' }))
    await expect(args.onSave).toHaveBeenCalledWith({
      seq: 5,
      type: 'decision',
      values: {
        summary: 'Notify the DPA within 72 hours. Done.',
        severity: 'high',
        owner: 'PER-7',
        notify_dpa: true,
        due: undefined,
      },
      rendered: ['summary', 'severity', 'owner', 'notify_dpa', 'due'],
    })
    await userEvent.click(canvas.getByRole('button', { name: 'Cancel' }))
    await expect(args.onCancelEdit).toHaveBeenCalled()
  },
}

/** Sending: the button keeps its place and swaps its label after the delay. */
export const Submitting: Story = {
  args: { submitting: true },
}

/** The composer's exposed methods, as a story harness calls them. */
interface ComposerHandle {
  reset: (event?: ChannelPostEvent | ChannelEditEvent) => void
  select: (type: string) => boolean
}

/** Two types that share a ref property name, and an optional boolean. */
const sharedRefTypes: ChannelMessageTypeDef[] = [
  {
    name: 'task',
    label: 'Task',
    persist: 'log',
    reply: false,
    properties: [
      { name: 'owner', label: 'Owner', type: 'ref', to: ['person'] },
      { name: 'urgent', label: 'Urgent', type: 'boolean' },
    ],
    actions: [],
  },
  {
    name: 'review',
    label: 'Review',
    persist: 'log',
    reply: false,
    properties: [{ name: 'owner', label: 'Owner', type: 'ref', to: ['person'] }],
    actions: [],
  },
]

/** A type whose required ref may hold an entity the editor cannot read. */
const assignmentType: ChannelMessageTypeDef = {
  name: 'assignment',
  label: 'Assignment',
  persist: 'log',
  reply: false,
  properties: [
    { name: 'assignee', label: 'Assignee', type: 'ref', required: true, to: ['person'] },
    { name: 'note', label: 'Note', type: 'string' },
  ],
  actions: [],
}

/** Plain Enter in a one-line field does not send; only Ctrl+Enter or the button does. */
export const EnterDoesNotSend: Story = {
  args: { postable: ['finding'] },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(canvas.getByRole('textbox', { name: /Title/ }), 'Half a finding{Enter}')
    await expect(args.onPost).not.toHaveBeenCalled()
    await userEvent.keyboard('{Control>}{Enter}{/Control}')
    await expect(args.onPost).toHaveBeenCalledWith(expect.objectContaining({ type: 'finding' }))
  },
}

/** An integer leaves as the text the user typed; parsing it is the app's. */
export const IntegerIsRaw: Story = {
  args: { postable: ['finding'] },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(canvas.getByRole('textbox', { name: /Title/ }), 'Leak')
    const affected = canvas.getByRole('textbox', { name: /Affected records/ })
    await expect(affected).toHaveAttribute('inputmode', 'numeric')
    await userEvent.type(affected, '007')
    await userEvent.click(canvas.getByRole('button', { name: 'Send' }))
    await expect(args.onPost).toHaveBeenCalledWith(
      expect.objectContaining({ values: expect.objectContaining({ affected: '007' }) }),
    )
  },
}

/** Switching type and back keeps what was typed in each. */
export const TypeSwitchKeepsDrafts: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(canvas.getByRole('textbox', { name: /Message/ }), 'Half a message')
    await userEvent.click(canvas.getByRole('radio', { name: 'Decision' }))
    await userEvent.type(canvas.getByRole('textbox', { name: /Decision/ }), 'Half a decision')
    await userEvent.click(canvas.getByRole('radio', { name: 'Message' }))
    await expect(canvas.getByRole('textbox', { name: /Message/ })).toHaveValue('Half a message')
    await userEvent.click(canvas.getByRole('radio', { name: 'Decision' }))
    await expect(canvas.getByRole('textbox', { name: /Decision/ })).toHaveValue('Half a decision')
  },
}

/**
 * A ref chosen on one type stays on that type: another type with a property
 * of the same name starts empty, and sends nothing for it.
 */
export const RefsPerType: Story = {
  args: { messageTypes: sharedRefTypes, postable: ['task', 'review'], presentation: undefined },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('button', { name: 'Choose…' }))
    await waitFor(() => expect(canvas.getByText('PER-7')).toBeVisible())
    await userEvent.click(canvas.getByRole('radio', { name: 'Review' }))
    await expect(canvas.queryByText('PER-7')).toBeNull()
    await userEvent.click(canvas.getByRole('button', { name: 'Send' }))
    await expect(args.onPost).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'review', values: { owner: undefined } }),
    )
    await userEvent.click(canvas.getByRole('radio', { name: 'Task' }))
    await expect(canvas.getByText('PER-7')).toBeVisible()
  },
}

/**
 * An optional box has one unchecked state, "not set": checking and
 * unchecking sends nothing rather than false.
 */
export const OptionalBoolean: Story = {
  args: { messageTypes: sharedRefTypes, postable: ['task'], presentation: undefined },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    const box = canvas.getByRole('checkbox', { name: /Urgent/ })
    await userEvent.click(box)
    await userEvent.click(box)
    await userEvent.click(canvas.getByRole('button', { name: 'Send' }))
    await expect(args.onPost).toHaveBeenCalledWith(
      expect.objectContaining({ values: { owner: undefined, urgent: undefined } }),
    )
  },
}

/** A picker the PickerInFlight story settles by hand. */
const deferredPicker = (() => {
  let settle: ((r: ChannelReadableRef | null) => void) | undefined
  let fail: ((e: Error) => void) | undefined
  const pickRef = fn(
    () =>
      new Promise<ChannelReadableRef | null>((resolve, reject) => {
        settle = resolve
        fail = reject
      }),
  )
  return {
    pickRef,
    settle: (r: ChannelReadableRef) => settle?.(r),
    fail: () => fail?.(new Error('dialog closed')),
  }
})()

/**
 * The picker runs once at a time, and a picker that fails is a cancel: the
 * field stays empty and the button works again.
 */
export const PickerInFlight: Story = {
  args: {
    messageTypes: sharedRefTypes,
    postable: ['review'],
    presentation: undefined,
    pickRef: deferredPicker.pickRef,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    deferredPicker.pickRef.mockClear()
    const choose = canvas.getByRole('button', { name: 'Choose…' })
    await userEvent.click(choose)
    await userEvent.click(choose)
    await expect(deferredPicker.pickRef).toHaveBeenCalledTimes(1)
    deferredPicker.fail()
    await waitFor(() => expect(choose).not.toHaveAttribute('aria-disabled', 'true'))
    await expect(canvas.queryByText('PER-7')).toBeNull()
    await userEvent.click(choose)
    await expect(deferredPicker.pickRef).toHaveBeenCalledTimes(2)
    deferredPicker.settle(pickedPerson)
    await waitFor(() => expect(canvas.getByText('PER-7')).toBeVisible())
  },
}

/** After a post lands, the app passes the event to `reset()` and the fields clear. */
export const ResetAfterPost: Story = {
  render: (args) => ({
    components: { RlChannelComposer },
    setup() {
      const composer = ref<ComposerHandle | null>(null)
      async function onPost(e: ChannelPostEvent) {
        // The answer arrives later; reset clears the posted type whatever is on screen.
        await new Promise((r) => setTimeout(r, 20))
        composer.value?.reset(e)
      }
      return { args, composer, onPost }
    },
    template: `
      <div style="max-width:640px">
        <RlChannelComposer ref="composer" v-bind="args" @post="onPost" />
      </div>`,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(canvas.getByRole('textbox', { name: /Message/ }), 'Sent.')
    await userEvent.click(canvas.getByRole('button', { name: 'Send' }))
    await userEvent.click(canvas.getByRole('radio', { name: 'Decision' }))
    await userEvent.type(canvas.getByRole('textbox', { name: /Decision/ }), 'Still writing')
    await userEvent.click(canvas.getByRole('radio', { name: 'Message' }))
    await waitFor(() => expect(canvas.getByRole('textbox', { name: /Message/ })).toHaveValue(''))
    await userEvent.click(canvas.getByRole('radio', { name: 'Decision' }))
    await expect(canvas.getByRole('textbox', { name: /Decision/ })).toHaveValue('Still writing')
  },
}

/**
 * Editing puts the new-message draft and the selected type aside and brings
 * both back when the edit ends. A rebuilt `editing` object for the same
 * message (a live update) keeps what the user typed, and a `reset()` the app
 * calls late after a save does not wipe the restored draft.
 */
export const EditStashesDraft: Story = {
  render: (args) => ({
    components: { RlChannelComposer },
    setup() {
      const editing = ref<ChannelEditing | undefined>()
      const composer = ref<ComposerHandle | null>(null)
      const start = () => {
        editing.value = { seq: 3, type: 'text', values: { body: 'Policy reverted.' } }
      }
      const rebuild = () => {
        editing.value = { ...editing.value!, values: { body: 'Policy reverted (server copy).' } }
      }
      async function onSave(e: ChannelEditEvent) {
        editing.value = undefined
        await new Promise((r) => setTimeout(r, 20))
        composer.value?.reset(e)
      }
      return { args, editing, composer, start, rebuild, onSave, end: () => (editing.value = undefined) }
    },
    template: `
      <div style="max-width:640px">
        <RlChannelComposer
          ref="composer"
          v-bind="args"
          :editing="editing"
          @save="onSave"
          @cancel-edit="end"
        />
        <button type="button" data-testid="start" @click="start">Start edit</button>
        <button type="button" data-testid="rebuild" @click="rebuild">Rebuild</button>
      </div>`,
  }),
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('radio', { name: 'Decision' }))
    await userEvent.type(canvas.getByRole('textbox', { name: /Decision/ }), 'Half a decision')

    await userEvent.click(canvas.getByTestId('start'))
    const body = await canvas.findByRole('textbox', { name: /Message/ })
    await expect(body).toHaveValue('Policy reverted.')
    await userEvent.type(body, ' Done.')
    await userEvent.click(canvas.getByTestId('rebuild'))
    await nextTick()
    await expect(canvas.getByRole('textbox', { name: /Message/ })).toHaveValue('Policy reverted. Done.')

    await userEvent.click(canvas.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(canvas.getByRole('radio', { name: 'Decision' })).toBeChecked())
    await expect(canvas.getByRole('textbox', { name: /Decision/ })).toHaveValue('Half a decision')

    await userEvent.click(canvas.getByTestId('start'))
    await userEvent.click(await canvas.findByRole('button', { name: 'Save' }))
    await expect(args.onSave).toHaveBeenCalledWith(expect.objectContaining({ seq: 3 }))
    await new Promise((r) => setTimeout(r, 50))
    await expect(canvas.getByRole('radio', { name: 'Decision' })).toBeChecked()
    await expect(canvas.getByRole('textbox', { name: /Decision/ })).toHaveValue('Half a decision')
  },
}

/**
 * A required ref the editor cannot read shows the restricted placeholder and
 * Replace, counts as filled, and stays out of the event so the server keeps
 * it. Picking a replacement sends the new one.
 */
export const EditRestrictedRef: Story = {
  args: {
    messageTypes: [assignmentType],
    postable: ['assignment'],
    presentation: undefined,
    editing: { seq: 9, type: 'assignment', values: { note: 'Before Friday' }, restricted: ['assignee'] },
  },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    const group = canvas.getByRole('group', { name: 'Assignee' })
    await expect(within(group).getByText('Restricted reference')).toBeVisible()
    await expect(within(group).queryByRole('button', { name: 'Clear' })).toBeNull()
    await userEvent.click(canvas.getByRole('button', { name: 'Save' }))
    await expect(canvas.queryByText('Required.')).toBeNull()
    await expect(args.onSave).toHaveBeenLastCalledWith({
      seq: 9,
      type: 'assignment',
      values: { note: 'Before Friday' },
      rendered: ['note'],
    })
    await userEvent.click(within(group).getByRole('button', { name: 'Replace…' }))
    await waitFor(() => expect(within(group).getByText('PER-7')).toBeVisible())
    await userEvent.click(canvas.getByRole('button', { name: 'Save' }))
    await expect(args.onSave).toHaveBeenLastCalledWith({
      seq: 9,
      type: 'assignment',
      values: { assignee: 'PER-7', note: 'Before Friday' },
      rendered: ['assignee', 'note'],
    })
  },
}

/** An edit of a message whose type was removed says so and can be cancelled. */
export const EditTypeGone: Story = {
  args: { editing: { seq: 4, type: 'vanished', values: {} } },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText(/no longer exists/)).toBeVisible()
    await userEvent.click(canvas.getByRole('button', { name: 'Cancel' }))
    await expect(args.onCancelEdit).toHaveBeenCalled()
  },
}

const freshTyping = fn()
const editTyping = fn()

/**
 * The composer emits `typing` while the user writes a new message of a log
 * type, picking a ref included. It does not for an ephemeral type such as Status, and not while
 * editing a message.
 */
export const EmitsTyping: Story = {
  render: (args) => ({
    components: { RlChannelComposer },
    setup: () => ({
      args,
      freshTyping,
      editTyping,
      editing: { seq: 5, type: 'text', values: { body: 'Old text.' } } satisfies ChannelEditing,
    }),
    template: `
      <div style="max-width:640px; display:grid; gap:16px">
        <RlChannelComposer v-bind="args" @typing="freshTyping" />
        <RlChannelComposer v-bind="args" :editing="editing" @typing="editTyping" />
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    freshTyping.mockClear()
    editTyping.mockClear()
    const [fresh, edit] = canvasElement.querySelectorAll('form')
    const canvas = within(fresh as HTMLElement)
    await userEvent.type(canvas.getByRole('textbox', { name: /Message/ }), 'Lo')
    await expect(freshTyping).toHaveBeenCalled()

    await userEvent.click(canvas.getByRole('radio', { name: 'Decision' }))
    freshTyping.mockClear()
    await userEvent.click(canvas.getByRole('button', { name: 'Choose a person…' }))
    await waitFor(() => expect(canvas.getByText('PER-7')).toBeVisible())
    await expect(freshTyping).toHaveBeenCalled()

    freshTyping.mockClear()
    await userEvent.click(canvas.getByRole('radio', { name: 'Status' }))
    await userEvent.type(canvas.getByRole('textbox', { name: /Status/ }), 'Away')
    await expect(freshTyping).not.toHaveBeenCalled()

    await userEvent.type(within(edit as HTMLElement).getByRole('textbox', { name: /Message/ }), ' More.')
    await expect(editTyping).not.toHaveBeenCalled()
  },
}

/**
 * The built-in typing indicator is not a message type: with typing on, the
 * schema lists no `typing` type, so the composer offers none.
 */
export const TypingIsNotAMessageType: Story = {
  play: async ({ canvasElement }) => {
    const group = within(canvasElement).getByRole('radiogroup', { name: 'Message type' })
    const options = within(group).getAllByRole('radio').map((r) => r.textContent?.trim())
    await expect(options).toEqual(['Message', 'Decision', 'Finding', 'Status'])
    await expect(options).not.toContain('Typing')
  },
}

/* Session transitions ---------------------------------------------------- */

const breachArgs = {
  messageTypes: breachMessageTypes,
  postable: ['text', 'contained', 'dismiss'],
  presentation: undefined,
  transitions: breachTransitions,
}

function typeOptions(canvasElement: HTMLElement): HTMLElement[] {
  return within(within(canvasElement).getByRole('radiogroup', { name: 'Message type' })).getAllByRole('radio')
}

function submitButton(canvasElement: HTMLElement): HTMLElement {
  return within(canvasElement).getByRole('button', { name: /Send/ })
}

/** The hint the send button refers to, or null. */
function describedHint(canvasElement: HTMLElement): string | null {
  const id = submitButton(canvasElement).getAttribute('aria-describedby')
  return id ? (canvasElement.querySelector(`[id="${id}"]`)?.textContent?.trim() ?? null) : null
}

/**
 * Types that move the session come first, marked with an icon and hidden
 * text; a plain message is still the default.
 */
export const TransitionTypesFirst: Story = {
  args: breachArgs,
  play: async ({ canvasElement }) => {
    const options = typeOptions(canvasElement)
    await expect(options.map((o) => o.querySelector('span')!.textContent)).toEqual(['Contained', 'Dismiss', 'Text'])
    for (const o of options.slice(0, 2)) {
      await expect(o).toHaveTextContent('(moves the session)')
      await expect(o.querySelector('svg')).not.toBeNull()
    }
    await expect(options[2]).not.toHaveTextContent('moves the session')
    await expect(options[2]).toHaveAttribute('aria-checked', 'true')
  },
}

/** Selecting a transition type says where the session goes; the button refers to it. */
export const TransitionHint: Story = {
  args: breachArgs,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(describedHint(canvasElement)).toBeNull()
    await userEvent.click(typeOptions(canvasElement)[0]!)
    await expect(canvas.getByText('Sending this moves the session to Contained.')).toBeVisible()
    await expect(describedHint(canvasElement)).toBe('Sending this moves the session to Contained.')
    await userEvent.click(typeOptions(canvasElement)[2]!)
    await expect(canvas.queryByText(/moves the session to/)).toBeNull()
    await expect(submitButton(canvasElement)).not.toHaveAttribute('aria-describedby')
  },
}

/** A guarded transition only moves the session if its condition holds. */
export const GuardedTransitionHint: Story = {
  args: {
    ...breachArgs,
    postable: ['text', 'assessed'],
    transitions: [{ messageType: 'assessed', toLabels: ['Risk assessed'], guarded: true }],
  },
  play: async ({ canvasElement }) => {
    await userEvent.click(typeOptions(canvasElement)[0]!)
    await expect(describedHint(canvasElement)).toBe(
      'Sending this moves the session to Risk assessed if its condition holds.',
    )
  },
}

/**
 * A type with several transitions may move the session to one of their
 * targets; which one depends on the message and the poster.
 */
export const BranchingTransitionHint: Story = {
  args: {
    ...breachArgs,
    postable: ['text', 'assessed'],
    transitions: [{ messageType: 'assessed', toLabels: ['Risk assessed', 'Contained'], guarded: true }],
  },
  play: async ({ canvasElement }) => {
    await userEvent.click(typeOptions(canvasElement)[0]!)
    await expect(describedHint(canvasElement)).toBe(
      'Sending this may move the session to Risk assessed or Contained.',
    )
  },
}

/**
 * One target, but only some transitions have a condition (say a lead's
 * entry has none and everyone else's has one): the hint says "may".
 */
export const MixedGuardTransitionHint: Story = {
  args: {
    ...breachArgs,
    postable: ['text', 'assessed'],
    transitions: [{ messageType: 'assessed', toLabels: ['Risk assessed'], mayMove: true }],
  },
  play: async ({ canvasElement }) => {
    await userEvent.click(typeOptions(canvasElement)[0]!)
    await expect(describedHint(canvasElement)).toBe('Sending this may move the session to Risk assessed.')
  },
}

/** Choosing another type reports it, so the app can drop what it tied to the old one. */
export const SelectEmits: Story = {
  args: breachArgs,
  play: async ({ args, canvasElement }) => {
    await userEvent.click(typeOptions(canvasElement)[2]!)
    await expect(args.onSelect).toHaveBeenLastCalledWith('text')
  },
}

/** When every offered type moves the session, the first is selected. */
export const OnlyTransitionTypes: Story = {
  args: { ...breachArgs, postable: ['contained', 'dismiss'] },
  play: async ({ canvasElement }) => {
    await expect(typeOptions(canvasElement)[0]).toHaveAttribute('aria-checked', 'true')
    await expect(describedHint(canvasElement)).toBe('Sending this moves the session to Contained.')
  },
}

/** One offered type that moves the session: no picker, so the hint is the only mark. */
export const OneTransitionType: Story = {
  args: { ...breachArgs, postable: ['contained'] },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).queryByRole('radiogroup')).toBeNull()
    await expect(describedHint(canvasElement)).toBe('Sending this moves the session to Contained.')
  },
}

/**
 * Someone else moves the session and the transitions change: the options
 * reorder, but the selection and focus stay on the same type.
 */
export const ReorderKeepsFocus: Story = {
  args: breachArgs,
  render: (args) => ({
    components: { RlChannelComposer },
    setup() {
      const transitions = ref(args.transitions)
      const change = () => {
        transitions.value = [{ messageType: 'text', toLabels: ['Contained'] }]
      }
      return { args, transitions, change }
    },
    template: `
      <div style="max-width:640px">
        <RlChannelComposer v-bind="args" :transitions="transitions" />
        <span data-testid="change" @click="change" />
      </div>`,
  }),
  play: async ({ canvasElement }) => {
    const text = typeOptions(canvasElement)[2]!
    text.focus()
    await expect(document.activeElement).toBe(text)
    ;(canvasElement.querySelector('[data-testid="change"]') as HTMLElement).click()
    await nextTick()
    await waitFor(() =>
      expect(typeOptions(canvasElement).map((o) => o.querySelector('span')!.textContent)).toEqual([
        'Text',
        'Contained',
        'Dismiss',
      ]),
    )
    const after = typeOptions(canvasElement)[0]!
    await expect(after).toHaveAttribute('aria-checked', 'true')
    await expect(document.activeElement).toBe(after)
  },
}

/** An edit never moves a session, so it shows no hint. */
export const EditHasNoHint: Story = {
  args: { ...breachArgs, editing: { seq: 2, type: 'contained', values: { note: 'Revoked.' } } },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).queryByText(/moves the session/)).toBeNull()
    await expect(within(canvasElement).getByRole('button', { name: 'Save' })).not.toHaveAttribute('aria-describedby')
  },
}

/** A reply of a transition type moves the session, so the hint shows. */
export const ReplyShowsHint: Story = {
  args: {
    ...breachArgs,
    postable: ['contained'],
    replyTo: { seq: 1, author: 'Marieke de Vries', excerpt: 'Revoking access now.' },
  },
  play: async ({ canvasElement }) => {
    await expect(describedHint(canvasElement)).toBe('Sending this moves the session to Contained.')
  },
}

/**
 * `select(type)` selects an offered type and focuses its first field. It
 * returns false, and changes nothing, for a type not offered or during an
 * edit.
 */
export const SelectByMethod: Story = {
  args: breachArgs,
  render: (args) => ({
    components: { RlChannelComposer },
    setup() {
      const composer = ref<ComposerHandle | null>(null)
      const editing = ref<ChannelEditing>()
      const answers = ref<boolean[]>([])
      const pick = (t: string) => answers.value.push(composer.value!.select(t))
      const edit = () => (editing.value = { seq: 9, type: 'text', values: { body: 'old' }, refs: {} })
      return { args, composer, editing, answers, pick, edit }
    },
    template: `
      <div style="max-width:640px">
        <RlChannelComposer ref="composer" v-bind="args" :editing="editing" />
        <button type="button" @click="pick('dismiss')">select dismiss</button>
        <button type="button" @click="pick('unknown')">select unknown</button>
        <button type="button" @click="edit()">start edit</button>
        <output data-testid="answers">{{ answers.join(',') }}</output>
      </div>`,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('button', { name: 'select dismiss' }))
    await expect(typeOptions(canvasElement)[1]).toHaveAttribute('aria-checked', 'true')
    await waitFor(() => expect(canvas.getByRole('textbox', { name: /Reason/ })).toHaveFocus())
    await userEvent.click(canvas.getByRole('button', { name: 'select unknown' }))
    await expect(typeOptions(canvasElement)[1]).toHaveAttribute('aria-checked', 'true')
    await userEvent.click(canvas.getByRole('button', { name: 'start edit' }))
    await nextTick()
    await userEvent.click(canvas.getByRole('button', { name: 'select dismiss' }))
    await expect(canvas.getByTestId('answers')).toHaveTextContent('true,false,false')
    await expect(canvas.getByRole('form', { name: 'Edit message' })).toBeVisible()
  },
}

/** Blocked by a decision in flight: the send button is disabled, with no pending label. */
export const Blocked: Story = {
  args: { blocked: true, onPost: fn() },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(canvas.getByRole('textbox', { name: /Message/ }), 'Wait.')
    await userEvent.keyboard('{Control>}{Enter}{/Control}')
    await expect(args.onPost).not.toHaveBeenCalled()
    const send = canvas.getByRole('button', { name: 'Send' })
    await expect(send).toBeDisabled()
    await expect(send).not.toHaveAttribute('aria-busy')
  },
}
