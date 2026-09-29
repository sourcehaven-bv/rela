import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import { ref } from 'vue'
import RlViewTabs from './RlViewTabs.vue'

const tabs = [
  { id: 'board', label: 'Board', icon: 'kanban' as const },
  { id: 'table', label: 'Table', icon: 'table' as const },
  { id: 'timeline', label: 'Timeline', icon: 'chart-no-axes-gantt' as const },
]

const meta: Meta<typeof RlViewTabs> = {
  title: 'Layout/View tabs',
  component: RlViewTabs,
  parameters: { layout: 'padded' },
  argTypes: { showAdd: { control: 'boolean' } },
  args: { tabs, modelValue: 'board', showAdd: true },
  render: (args) => ({
    components: { RlViewTabs },
    setup: () => ({ args, current: ref(args.modelValue) }),
    template: '<RlViewTabs v-bind="args" v-model="current" />',
  }),
}
export default meta
type Story = StoryObj<typeof RlViewTabs>

/** Left and right arrows move between tabs, as expected of a tablist. */
export const Playground: Story = {}

export const WithoutAddButton: Story = { args: { showAdd: false } }

/**
 * A view with its own URL should be a real link: only an anchor gives the
 * middle click, the modifier click, the context menu and the status bar a
 * user expects from navigation.
 *
 * Per tab, so a row can mix a view that is a URL with one that is not — the
 * "Add" control stays a button here, because it creates rather than goes.
 *
 * `href` is used below; a router link is passed the same way, as the
 * component itself in `as` with its `to` in `attrs`, so this library takes no
 * dependency on a router.
 */
export const LinkTabs: Story = {
  args: {
    tabs: [
      { id: 'board', label: 'Board', icon: 'kanban' as const, as: 'a', attrs: { href: '#board' } },
      { id: 'table', label: 'Table', icon: 'table' as const, as: 'a', attrs: { href: '#table' } },
      { id: 'notes', label: 'Notes', icon: 'note' as const },
    ],
    modelValue: 'board',
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    /* The linked tabs are anchors carrying a real href, not buttons. */
    const board = canvas.getByRole('tab', { name: 'Board' })
    await expect(board.tagName).toBe('A')
    await expect(board).toHaveAttribute('href', '#board')

    /* A tab that is not a link stays a button, in the same row. */
    await expect(canvas.getByRole('tab', { name: 'Notes' }).tagName).toBe('BUTTON')

    /*
     * Arrow keys still move between them, and focus follows the selection:
     * with links the two come apart, and leaving focus behind would make the
     * next arrow press move from the wrong tab.
     */
    board.focus()
    await userEvent.keyboard('{ArrowRight}')

    const table = canvas.getByRole('tab', { name: 'Table' })
    await expect(table).toHaveAttribute('aria-selected', 'true')
    await expect(table).toHaveFocus()
  },
  render: (args) => ({
    components: { RlViewTabs },
    setup: () => ({ args, current: ref(args.modelValue) }),
    template: '<RlViewTabs v-bind="args" v-model="current" />',
  }),
}
