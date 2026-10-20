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
      f.querySelector('.rl-field__label, .rl-field__wrapped-label')?.textContent?.replace('*', '').trim(),
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
