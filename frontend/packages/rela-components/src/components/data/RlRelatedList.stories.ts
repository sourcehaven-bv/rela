import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, fn, userEvent, within } from 'storybook/test'
import { h } from 'vue'
import RlRelatedList from './RlRelatedList.vue'
import RlIcon from '../common/RlIcon.vue'
import type { IconName } from '../common/icons'
import type { RelatedItem } from './types'
import { storyComponent } from '../storyGeneric'

const icon = (name: IconName) => () => h(RlIcon, { name })

const subtasks: RelatedItem[] = [
  {
    id: 's1',
    title: 'Give awesome feedback on UX for Atlas Projects',
    icon: icon('check'),
    iconLabel: 'Done',
    meta: [
      { id: 'due', label: 'Due', value: 'Aug 23', tone: 'success' },
      { id: 'assignee', label: 'Assigned to', value: 'Rowdy', tone: 'success' },
    ],
  },
  {
    id: 's2',
    title: 'Give awesome feedback on UX for Atlas Projects',
    icon: icon('progress'),
    iconLabel: 'In progress',
    meta: [
      { id: 'due', label: 'Due', value: 'Aug 27' },
      { id: 'assignee', label: 'Assigned to', value: 'Jeroen' },
    ],
  },
]

/* The list is generic over its item; `RelatedItem` is the plain shape. */
const meta: Meta<typeof RlRelatedList<RelatedItem>> = {
  title: 'Data/Related list',
  component: storyComponent(RlRelatedList),
  parameters: { layout: 'padded' },
  args: { title: 'Subtasks', items: subtasks, addLabel: 'Add subtask', onSelect: fn(), onAdd: fn() },
  decorators: [() => ({ template: '<div style="max-width:640px"><story /></div>' })],
}
export default meta
type Story = StoryObj<typeof RlRelatedList<RelatedItem>>

export const Playground: Story = {
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByRole('heading', { name: /Subtasks/ })).toHaveTextContent('· 2')
    // The glyph's meaning and each value's name reach a screen reader.
    const first = canvas.getAllByRole('button', { name: /Give awesome feedback/ })[0]
    await expect(first).toHaveAccessibleName('Done: Give awesome feedback on UX for Atlas Projects')
    await expect(canvas.getAllByText('Due', { exact: false })[0]).toHaveClass('rl-visually-hidden')
    await userEvent.click(first)
    await expect(args.onSelect).toHaveBeenCalledWith(subtasks[0])
    await userEvent.click(canvas.getByRole('button', { name: 'Add subtask' }))
    await expect(args.onAdd).toHaveBeenCalled()
  },
}

/**
 * Not only subtasks. Rows without a glyph or values are just titles, and a
 * row that navigates renders as a link.
 */
export const OtherRelations: Story = {
  args: {
    title: 'Mitigated by',
    addLabel: 'Link measure',
    items: [
      { id: 'm1', title: 'Encrypt backups at rest', as: 'a', attrs: { href: '#m1' } },
      {
        id: 'm2',
        title: 'Quarterly access review',
        as: 'a',
        attrs: { href: '#m2' },
        meta: [{ id: 'owner', label: 'Owner', value: 'Security officer' }],
      },
      {
        id: 'm3',
        title: 'Restore drill',
        icon: icon('warning'),
        iconLabel: 'Overdue',
        iconTone: 'danger',
        as: 'a',
        attrs: { href: '#m3' },
        meta: [{ id: 'due', label: 'Due', value: 'Sep 1', tone: 'danger' }],
      },
    ],
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByRole('link', { name: 'Encrypt backups at rest' })).toHaveAttribute('href', '#m1')
  },
}

/** An empty list says so, rather than drawing a heading over nothing. */
export const Empty: Story = {
  args: { items: [], emptyLabel: 'No subtasks yet.' },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('No subtasks yet.')).toBeVisible()
  },
}

/** Read-only: no add label, no button. */
export const ReadOnly: Story = {
  args: { addLabel: undefined },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.queryByRole('button', { name: 'Add subtask' })).toBeNull()
  },
}

/** The `row` slot replaces a row's drawing and keeps the heading and button. */
export const CustomRow: Story = {
  render: (args) => ({
    components: { RlRelatedList },
    setup: () => ({ args }),
    template: `
      <RlRelatedList v-bind="args">
        <template #row="{ item }">
          <div style="padding:8px; border-bottom:1px solid var(--rl-color-border)">
            <strong>{{ item.id }}</strong> {{ item.title }}
          </div>
        </template>
      </RlRelatedList>
    `,
  }),
}
