import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, waitFor, within } from 'storybook/test'
import { ref } from 'vue'
import RlCommandPalette, { type Command } from './RlCommandPalette.vue'
import RlButton from '../common/RlButton.vue'

const meta: Meta<typeof RlCommandPalette> = {
  title: 'Overlay/Command palette',
  component: RlCommandPalette,
  parameters: { layout: 'fullscreen' },
}
export default meta

const commands: Command[] = [
  { id: 'new-task', label: 'New task', icon: 'add', keys: 'N', group: 'Create' },
  { id: 'new-doc', label: 'New document', icon: 'document', group: 'Create' },
  { id: 'board', label: 'Go to board', icon: 'kanban', keys: 'G then B', group: 'Go to' },
  { id: 'timeline', label: 'Go to timeline', icon: 'gantt', group: 'Go to' },
  { id: 'my-tasks', label: 'Go to my tasks', icon: 'done', group: 'Go to' },
  {
    id: 'theme',
    label: 'Toggle dark mode',
    description: 'Switches the workspace theme',
    icon: 'moon',
    group: 'Settings',
  },
  {
    id: 'archive',
    label: 'Archive initiative',
    description: 'Only an owner can archive',
    icon: 'archive',
    group: 'Settings',
    disabled: true,
  },
]

function setup() {
  const open = ref(true)
  const ran = ref<string | null>(null)
  return { open, commands, ran, run: (c: Command) => (ran.value = c.label) }
}

const template = `
  <div style="height:100vh; padding:24px; background:var(--rl-color-bg)">
    <RlButton @click="open = true">Open palette</RlButton>
    <p v-if="ran" data-testid="ran">Ran: {{ ran }}</p>
    <RlCommandPalette v-model:open="open" :commands="commands" @run="run" />
  </div>
`

/**
 * Type to narrow, arrow to move, Enter to run. Anchored to the top, which is
 * where a dialog you type into belongs: centred, the list grows both ways
 * and the eye has to chase it.
 */
export const Default: StoryObj = {
  render: () => ({ components: { RlCommandPalette, RlButton }, setup, template }),
}

/**
 * The keyboard path, which is the only path most users take: the query
 * narrows the list, the arrows move a cursor that wraps, and Enter runs what
 * the cursor is on.
 */
export const KeyboardDriven: StoryObj = {
  play: async () => {
    /*
     * Queried from the body, not the canvas: the modal the palette is built
     * on teleports out of the story root, so a canvas-scoped query sees the
     * trigger button and nothing else.
     */
    const canvas = within(document.body)
    const input = await canvas.findByRole('combobox')

    /* The modal hands focus to the field, so typing works without a click. */
    await waitFor(() => expect(input).toHaveFocus())

    await userEvent.type(input, 'go to')
    await waitFor(() => expect(canvas.getAllByRole('option')).toHaveLength(3))

    /*
     * Asserted through `aria-selected` rather than the highlight class: the
     * class is how it looks, and this is what a screen reader is told.
     */
    await expect(canvas.getAllByRole('option')[0]).toHaveAttribute('aria-selected', 'true')

    await userEvent.keyboard('{ArrowDown}')
    await expect(canvas.getAllByRole('option')[1]).toHaveAttribute('aria-selected', 'true')

    /* Up from the top wraps to the end rather than stopping. */
    await userEvent.keyboard('{ArrowUp}{ArrowUp}')
    await expect(canvas.getAllByRole('option')[2]).toHaveAttribute('aria-selected', 'true')

    await userEvent.keyboard('{Enter}')
    await expect(await canvas.findByTestId('ran')).toHaveTextContent('Go to my tasks')

    /* Running a command closes the palette; it described a thing now done. */
    await waitFor(() => expect(canvas.queryByRole('combobox')).toBeNull())
  },
  render: () => ({ components: { RlCommandPalette, RlButton }, setup, template }),
}

/** A query matching nothing says so rather than showing an empty box. */
export const NoMatches: StoryObj = {
  play: async () => {
    /*
     * Queried from the body, not the canvas: the modal the palette is built
     * on teleports out of the story root, so a canvas-scoped query sees the
     * trigger button and nothing else.
     */
    const canvas = within(document.body)
    const input = await canvas.findByRole('combobox')
    await userEvent.type(input, 'zzzz')

    await expect(await canvas.findByText('No matching commands')).toBeInTheDocument()
    await expect(canvas.queryAllByRole('option')).toHaveLength(0)

    /* Enter on nothing is a no-op rather than an error. */
    await userEvent.keyboard('{Enter}')
    await expect(canvas.queryByTestId('ran')).toBeNull()
  },
  render: () => ({ components: { RlCommandPalette, RlButton }, setup, template }),
}
