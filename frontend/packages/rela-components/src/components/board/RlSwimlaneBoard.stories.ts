import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import RlSwimlaneBoard from './RlSwimlaneBoard.vue'
import RlSwimlane from './RlSwimlane.vue'
import RlTaskCard from './RlTaskCard.vue'
import type { Task } from '../../types'
import { storyComponent } from '../storyGeneric'
import { swimlanes } from '../../fixtures'
import RlIcon from '../common/RlIcon.vue'
import type { IconName } from '../common/icons'
import { h } from 'vue'
import { ref } from 'vue'
import type { Section, Swimlane } from '../../types'

/* Generic over its item, as `RlBoard` is; `Task` is this library's demo row. */
const meta: Meta<typeof RlSwimlaneBoard<Task>> = {
  title: 'Board/Swimlane board',
  component: storyComponent(RlSwimlaneBoard),
}
export default meta

type Story = StoryObj<typeof RlSwimlaneBoard<Task>>

/** The same cards as the board's, grouped a second time by who holds them. */
export const Default: Story = {
  args: { lanes: swimlanes, addLabel: 'Add task' },
  render: (args) => ({
    components: { RlSwimlaneBoard: storyComponent(RlSwimlaneBoard), RlTaskCard },
    setup: () => ({ args }),
    template: `
      <RlSwimlaneBoard v-bind="args">
        <template #card="{ item, selected }">
          <RlTaskCard :task="item" :selected="selected" />
        </template>
      </RlSwimlaneBoard>
    `,
  }),
}

/** Selection is marked in the lane it falls in, as on the board. */
export const Selected: Story = {
  ...Default,
  args: { lanes: swimlanes, addLabel: 'Add task', selectedId: 'q4' },
}

/**
 * A collapsed lane keeps its header, so a board can be narrowed to the lanes
 * being worked on without losing the ones that are not.
 */
export const CollapsedLane: Story = {
  ...Default,
  args: {
    lanes: swimlanes.map((lane) => (lane.id === 'alice' ? { ...lane, collapsed: true } : lane)),
    addLabel: 'Add task',
  },
}

/** One lane on its own, which is what the board stacks. */
export const Lane: StoryObj = {
  parameters: { layout: 'padded' },
  render: () => ({
    components: { RlSwimlane: storyComponent(RlSwimlane), RlTaskCard },
    setup: () => ({ lane: swimlanes[0] }),
    template: `
      <RlSwimlane :lane="lane" add-label="Add task">
        <template #card="{ item, selected }">
          <RlTaskCard :task="item" :selected="selected" />
        </template>
      </RlSwimlane>
    `,
  }),
}

/**
 * Drag a card to another cell and the board reports it; this story holds the
 * lanes in a ref and applies the move, which is what a consuming app does.
 *
 * Every card is movable here. `can-move` is per item, so a real board answers
 * it from whether the user may edit that card.
 */
export const DragAndDrop: StoryObj = {
  /* The two-axis keyboard move: arrows cross columns and lanes. */
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const card = canvas.getAllByRole('button', { name: /Press Enter to pick up/ })[0]

    card.focus()
    await userEvent.keyboard('{Enter}')
    await expect(card).toHaveClass(/rl-board-card--grabbed/)

    /* Right crosses a column, down crosses a lane; the cell lit is the one both name. */
    await userEvent.keyboard('{ArrowRight}{ArrowDown}')
    await expect(canvasElement.querySelectorAll('.rl-swimlane-column--target')).toHaveLength(1)

    await userEvent.keyboard('{Enter}')
    await expect(canvasElement).toHaveTextContent(/→ In progress, Alex/)
  },
  render: () => ({
    components: { RlSwimlaneBoard: storyComponent(RlSwimlaneBoard), RlTaskCard },
    setup() {
      const lanes = ref<Swimlane<Task>[]>(structuredClone(swimlanes))
      const moved = ref('')

      function onMove({ item, to, lane }: { item: Task; to: Section<Task>; lane: Swimlane<Task> }) {
        for (const l of lanes.value) {
          for (const s of l.sections) {
            s.items = s.items.filter((candidate) => candidate.id !== item.id)
          }
        }
        const target = lanes.value
          .find((l) => l.id === lane.id)
          ?.sections.find((s) => s.id === to.id)
        target?.items.push(item)
        moved.value = `${item.title} → ${to.title}, ${lane.title}`
      }

      return { lanes, moved, onMove, canMove: () => true }
    },
    template: `
      <div style="display:flex; flex-direction:column; height:100%">
        <p style="margin:0; padding:8px 24px; font: 12px system-ui; color:#666">
          {{ moved || 'Drag a card, or focus one and press Enter then the arrow keys.' }}
        </p>
        <RlSwimlaneBoard :lanes="lanes" :can-move="canMove" add-label="Add task" @move="onMove">
          <template #card="{ item, selected }">
            <RlTaskCard :task="item" :selected="selected" />
          </template>
        </RlSwimlaneBoard>
      </div>
    `,
  }),
}

/**
 * A column and a lane can each carry a glyph before the title, given as the
 * component itself so an app's own icon registry supplies it.
 *
 * The collapsed column keeps its glyph on the rail, which is why the icon
 * replaces the status dot rather than joining it.
 */
export const Icons: StoryObj = {
  render: () => ({
    components: { RlSwimlaneBoard: storyComponent(RlSwimlaneBoard), RlTaskCard },
    setup() {
      const icon = (name: IconName) => () => h(RlIcon, { name })
      const columnIcons: Record<string, IconName> = {
        backlog: 'inbox',
        postpone: 'clock',
        'in-progress': 'wrench',
        qa: 'done',
      }
      const laneIcons: Record<string, IconName> = {
        hanry: 'user',
        alex: 'user',
        alice: 'user',
        unassigned: 'users',
      }
      const lanes = swimlanes.map((lane) => ({
        ...lane,
        icon: icon(laneIcons[lane.id] ?? 'user'),
        sections: lane.sections.map((section) => ({
          ...section,
          icon: icon(columnIcons[section.id] ?? 'inbox'),
        })),
      }))
      return { lanes }
    },
    template: `
      <RlSwimlaneBoard :lanes="lanes" add-label="Add task">
        <template #card="{ item, selected }">
          <RlTaskCard :task="item" :selected="selected" />
        </template>
      </RlSwimlaneBoard>
    `,
  }),
}
