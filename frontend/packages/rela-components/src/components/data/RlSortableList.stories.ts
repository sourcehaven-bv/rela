import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import { expect, userEvent, within } from 'storybook/test'
import RlSortableList from './RlSortableList.vue'
import RlStatusDot from '../common/RlStatusDot.vue'
import RlText from '../common/RlText.vue'
import RlSwitch from '../form/RlSwitch.vue'
import { storyComponent } from '../storyGeneric'
import type { CollectionItem, StatusColor } from '../../types'

const meta: Meta = {
  title: 'Data/Sortable list',
  component: storyComponent(RlSortableList),
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj

/** The caller owns the order: a move arrives as an event and is applied here. */
function reorder<T>(items: T[], item: T, toIndex: number): T[] {
  const rest = items.filter((entry) => entry !== item)
  rest.splice(toIndex, 0, item)
  return rest
}

const columns: CollectionItem[] = [
  { id: 'title', title: 'Title' },
  { id: 'status', title: 'Status' },
  { id: 'priority', title: 'Priority' },
  { id: 'effort', title: 'Effort' },
  { id: 'assignee', title: 'Assignee' },
]

/**
 * Drag a handle, or Tab to one and press Space. While a row is held the arrow
 * keys move it, Enter puts it down and Escape puts it back. Nothing reaches
 * the caller until the row is put down.
 */
export const Playground: Story = {
  render: () => ({
    components: { RlSortableList },
    setup() {
      const items = ref(columns)
      const move = (item: CollectionItem, to: number) => (items.value = reorder(items.value, item, to))
      return { items, move }
    },
    template: `
      <div style="max-width:360px">
        <RlSortableList :items="items" label="List columns" @move="move" />
        <p data-testid="order" style="font-size:12px; color:var(--rl-color-text-muted)">{{ items.map((i) => i.id).join(', ') }}</p>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const order = canvas.getByTestId('order')

    // Pick Title up, move it down twice, and put it down.
    const handle = canvas.getByRole('button', { name: 'Reorder Title, 1 of 5' })
    handle.focus()
    await userEvent.keyboard(' ')
    await expect(canvas.getByRole('status')).toHaveTextContent('Title, picked up, 1 of 5')
    await userEvent.keyboard('{ArrowDown}{ArrowDown}')
    // The preview moves; the caller's order has not changed yet.
    await expect(order).toHaveTextContent('title, status, priority')
    await userEvent.keyboard('{Enter}')
    await expect(order).toHaveTextContent('status, priority, title, effort, assignee')
    await expect(canvas.getByRole('status')).toHaveTextContent('Title, dropped at 3 of 5')

    // Escape puts a held row back and emits nothing.
    const effort = canvas.getByRole('button', { name: 'Reorder Effort, 4 of 5' })
    effort.focus()
    await userEvent.keyboard(' {Home}{Escape}')
    await expect(order).toHaveTextContent('status, priority, title, effort, assignee')
    await expect(document.activeElement).toBe(canvas.getByRole('button', { name: 'Reorder Effort, 4 of 5' }))
  },
}

interface ChoiceValue extends CollectionItem {
  color: StatusColor
  isDefault?: boolean
}

/**
 * The row is the caller's: anything after the handle keeps its own clicks.
 * Here each value of a choice list shows its colour and a default switch.
 */
export const RichRows: Story = {
  render: () => ({
    components: { RlSortableList, RlStatusDot, RlText, RlSwitch },
    setup() {
      const items = ref<ChoiceValue[]>([
        { id: 'backlog', title: 'Backlog', color: 'grey', isDefault: true },
        { id: 'ready', title: 'Ready', color: 'blue' },
        { id: 'in-progress', title: 'In progress', color: 'amber' },
        { id: 'review', title: 'Review', color: 'amber' },
        { id: 'done', title: 'Done', color: 'green' },
      ])
      const move = (item: ChoiceValue, to: number) => (items.value = reorder(items.value, item, to))
      const setDefault = (id: string) => items.value.forEach((value) => (value.isDefault = value.id === id))
      return { items, move, setDefault }
    },
    template: `
      <div style="max-width:420px">
        <RlSortableList :items="items" label="Status values" @move="move">
          <template #item="{ item }">
            <div style="display:flex; align-items:center; gap:8px">
              <RlStatusDot :color="item.color" />
              <RlText size="sm" style="flex:1">{{ item.title }}</RlText>
              <RlSwitch :model-value="!!item.isDefault" label="Default" size="sm" @update:model-value="setDefault(item.id)" />
            </div>
          </template>
        </RlSortableList>
      </div>
    `,
  }),
}

/** Read-only: the order is shown, the handles are off. */
export const Disabled: Story = {
  render: () => ({
    components: { RlSortableList },
    setup: () => ({ items: columns }),
    template: `<div style="max-width:360px"><RlSortableList :items="items" label="List columns" disabled /></div>`,
  }),
}
