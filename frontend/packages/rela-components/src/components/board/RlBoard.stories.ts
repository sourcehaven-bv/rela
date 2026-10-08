import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import RlBoard from './RlBoard.vue'
import RlTaskCard from './RlTaskCard.vue'
import type { Task } from '../../types'
import { storyComponent } from '../storyGeneric'
import { boardSections } from '../../fixtures'
import { ref } from 'vue'
import type { Section } from '../../types'
import type { BoardDropPosition } from '../../composables/useBoardDnd'

/* The board is generic over its item; `Task` is this library's demo row. */
const meta: Meta<typeof RlBoard<Task>> = {
  title: 'Board/Board',
  component: storyComponent(RlBoard),
}
export default meta

type Story = StoryObj<typeof RlBoard<Task>>

/** The task card comes through the `card` slot, forwarded to every column. */
export const Default: Story = {
  args: { sections: boardSections, addLabel: 'Add task' },
  render: (args) => ({
    components: { RlBoard: storyComponent(RlBoard), RlTaskCard },
    setup: () => ({ args }),
    template: `
      <RlBoard v-bind="args">
        <template #card="{ item, selected }">
          <RlTaskCard :task="item" :selected="selected" />
        </template>
      </RlBoard>
    `,
  }),
}

export const Card: StoryObj = {
  parameters: { layout: 'padded' },
  render: () => ({
    components: { RlTaskCard },
    setup: () => ({ tasks: boardSections[0].items }),
    template: `
      <div style="display:flex; flex-direction:column; gap:8px; width:296px">
        <RlTaskCard v-for="t in tasks" :key="t.id" :task="t" />
      </div>
    `,
  }),
}

/**
 * A card can be dragged to another column, or moved with the keyboard: focus
 * a card, press Enter to pick it up, the arrow keys to choose a column and
 * Enter again to drop it.
 *
 * The board reports the move and changes nothing itself; this story applies
 * it to its own copy of the data, which is what a consuming app does.
 */
export const DragAndDrop: StoryObj = {
  /*
   * Drives the keyboard path for real, because it is the half of moving a
   * card that has no pointer to fall back on and the half most likely to
   * rot: nothing about a board looks broken when its keyboard move stops
   * working.
   */
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const card = canvas.getAllByRole('button', { name: /Press Enter to pick up/ })[0]

    card.focus()
    await userEvent.keyboard('{Enter}')
    await expect(card).toHaveClass(/rl-board-card--grabbed/)

    /*
     * Asserted on the column that lights up rather than on the live region:
     * the announcement is transient by design, so a test that waits on its
     * text is racing the thing that clears it.
     */
    await userEvent.keyboard('{ArrowRight}')
    await expect(canvasElement.querySelector('.rl-board-column--target')).toHaveTextContent(
      'In progress',
    )

    await userEvent.keyboard('{Enter}')
    await expect(canvasElement).toHaveTextContent(/→ In progress/)
  },
  render: () => ({
    components: { RlBoard: storyComponent(RlBoard), RlTaskCard },
    setup() {
      const sections = ref<Section<Task>[]>(structuredClone(boardSections))
      const moved = ref('')

      function onMove({ item, to }: { item: Task; to: Section<Task> }) {
        for (const section of sections.value) {
          section.items = section.items.filter((candidate) => candidate.id !== item.id)
        }
        sections.value.find((section) => section.id === to.id)?.items.push(item)
        moved.value = `${item.title} → ${to.title}`
      }

      return { sections, moved, onMove, canMove: () => true }
    },
    template: `
      <div style="display:flex; flex-direction:column; height:100%">
        <p style="margin:0; padding:8px 24px; font: 12px system-ui; color:#666">
          {{ moved || 'Drag a card, or focus one and press Enter then the arrow keys.' }}
        </p>
        <RlBoard :sections="sections" :can-move="canMove" add-label="Add task" @move="onMove">
          <template #card="{ item, selected }">
            <RlTaskCard :task="item" :selected="selected" />
          </template>
        </RlBoard>
      </div>
    `,
  }),
}

/**
 * Cards in an order the reader sets by hand. With `reorder`, a drop against a
 * card reports where it landed (`at`), and a card dropped among the cards of
 * its own column is reported too.
 *
 * As with a column move, the board changes nothing itself; this story moves
 * the card in its own copy of the data. The keyboard move still picks a
 * column only.
 */
export const Reorder: StoryObj = {
  render: () => ({
    components: { RlBoard: storyComponent(RlBoard), RlTaskCard },
    setup() {
      const sections = ref<Section<Task>[]>(structuredClone(boardSections))
      const moved = ref('')

      function onMove({ item, to, at }: { item: Task; to: Section<Task>; at?: BoardDropPosition }) {
        for (const section of sections.value) {
          section.items = section.items.filter((candidate) => candidate.id !== item.id)
        }
        const target = sections.value.find((section) => section.id === to.id)
        if (!target) return
        const index = at ? target.items.findIndex((candidate) => candidate.id === at.targetId) : -1
        if (index < 0) target.items.push(item)
        else target.items.splice(at?.placement === 'after' ? index + 1 : index, 0, item)
        moved.value = at ? `${item.title} ${at.placement} ${at.targetId}` : `${item.title} → ${to.title}`
      }

      return { sections, moved, onMove, canMove: () => true }
    },
    template: `
      <div style="display:flex; flex-direction:column; height:100%">
        <p style="margin:0; padding:8px 24px; font: 12px system-ui; color:#666">
          {{ moved || 'Drag a card above or below another one.' }}
        </p>
        <RlBoard :sections="sections" :can-move="canMove" reorder add-label="Add task" @move="onMove">
          <template #card="{ item, selected }">
            <RlTaskCard :task="item" :selected="selected" />
          </template>
        </RlBoard>
      </div>
    `,
  }),
}

/**
 * A card that is a link, which is the common shape: the whole card navigates
 * to the item.
 *
 * A link drags itself, so without help the browser would drag the anchor and
 * the card's own drag would never start. `RlBoardCard` marks links and images
 * in the slot `draggable="false"` so the gesture reaches the card, which is
 * why this needs nothing from the caller. An anchor that sets the attribute
 * itself is left alone.
 */
export const LinkCard: StoryObj = {
  play: async ({ canvasElement }) => {
    const anchors = canvasElement.querySelectorAll('.rl-board-card a')
    await expect(anchors.length).toBeGreaterThan(0)
    for (const anchor of anchors) {
      await expect(anchor).toHaveAttribute('draggable', 'false')
    }
  },
  render: () => ({
    components: { RlBoard: storyComponent(RlBoard), RlTaskCard },
    setup() {
      const sections = ref<Section<Task>[]>(structuredClone(boardSections))
      const moved = ref('')

      function onMove({ item, to }: { item: Task; to: Section<Task> }) {
        for (const section of sections.value) {
          section.items = section.items.filter((candidate) => candidate.id !== item.id)
        }
        sections.value.find((section) => section.id === to.id)?.items.push(item)
        moved.value = `${item.title} → ${to.title}`
      }

      return { sections, moved, onMove, canMove: () => true }
    },
    template: `
      <div style="display:flex; flex-direction:column; height:100%">
        <p style="margin:0; padding:8px 24px; font: 12px system-ui; color:#666">
          {{ moved || 'Every card is a link. Drag one anyway.' }}
        </p>
        <RlBoard :sections="sections" :can-move="canMove" add-label="Add task" @move="onMove">
          <template #card="{ item, selected }">
            <a :href="'#' + item.id" style="display:block; text-decoration:none; color:inherit">
              <RlTaskCard :task="item" :selected="selected" />
            </a>
          </template>
        </RlBoard>
      </div>
    `,
  }),
}
