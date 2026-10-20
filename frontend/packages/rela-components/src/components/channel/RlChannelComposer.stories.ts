import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, fn, userEvent, waitFor, within } from 'storybook/test'
import RlChannelComposer from './RlChannelComposer.vue'
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
