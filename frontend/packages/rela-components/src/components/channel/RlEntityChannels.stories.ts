import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import { expect, fn, userEvent, within } from 'storybook/test'
import RlEntityChannels from './RlEntityChannels.vue'
import { channelGroups } from '../../fixtures/channels'

const meta: Meta<typeof RlEntityChannels> = {
  title: 'Channel/Entity channels',
  component: RlEntityChannels,
  parameters: { layout: 'padded' },
  args: {
    groups: channelGroups,
    activeKey: 'incident',
    onSelect: fn(),
    onCreate: fn(),
  },
  render: (args) => ({
    components: { RlEntityChannels },
    setup: () => ({ args, filter: ref<'open' | 'all'>('all') }),
    template: '<div style="max-width:420px"><RlEntityChannels v-bind="args" v-model:filter="filter" /></div>',
  }),
}
export default meta
type Story = StoryObj<typeof RlEntityChannels>

/**
 * The channels the user may read, by type. Only a `many` type the user may
 * create offers a create button; a `one` type has its channel or nothing.
 */
export const Playground: Story = {
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByRole('button', { name: /Incident room/ })).toHaveAttribute('aria-current', 'true')
    await expect(canvas.getAllByRole('button', { name: /New/ })).toHaveLength(1)
    await userEvent.click(canvas.getByRole('button', { name: 'New topic' }))
    await expect(args.onCreate).toHaveBeenCalledWith('topic')
    await userEvent.click(canvas.getByRole('button', { name: /Forensics/ }))
    await expect(args.onSelect).toHaveBeenCalledWith('topic/forensics')
    await expect(canvas.getByText('No channels yet.')).toBeVisible()
  },
}

/** Titles are text, whoever chose them. */
export const MarkupInTitle: Story = {
  args: {
    groups: [
      {
        type: 'topic',
        label: 'Topics',
        cardinality: 'many',
        canCreate: false,
        channels: [{ key: 'x', title: '<img src=x onerror="alert(1)">', state: 'open' }],
      },
    ],
  },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelector('img')).toBeNull()
    await expect(within(canvasElement).queryByRole('button', { name: /New/ })).toBeNull()
  },
}

/**
 * Rows with unread messages show the count beside the title. The server
 * stops counting at 100, so 100 shows as `99+`; the row's name still says
 * the number.
 */
export const Unread: Story = {
  args: {
    activeKey: undefined,
    groups: [
      {
        type: 'topic',
        label: 'Topics',
        cardinality: 'many',
        canCreate: false,
        channels: [
          { key: 'topic/a', title: 'Containment', state: 'open', unread: 2 },
          { key: 'topic/b', title: 'Forensics', state: 'open', unread: 100 },
          { key: 'topic/c', title: 'Press', state: 'open', unread: 0 },
          { key: 'topic/d', title: 'Lessons', state: 'closed' },
        ],
      },
    ],
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const two = canvas.getByRole('button', { name: /Containment/ })
    await expect(two).toHaveAccessibleName(/2 unread/)
    await expect(within(two).getByText('2')).toBeVisible()
    const many = canvas.getByRole('button', { name: /Forensics/ })
    await expect(many).toHaveAccessibleName(/100 unread/)
    await expect(within(many).getByText('99+')).toBeVisible()
    await expect(canvas.getByRole('button', { name: /Press/ })).not.toHaveAccessibleName(/unread/)
    await expect(canvas.getByRole('button', { name: /Lessons/ })).not.toHaveAccessibleName(/unread/)
  },
}
