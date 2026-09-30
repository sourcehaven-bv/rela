import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlEditorToolbar from './RlEditorToolbar.vue'
import RlBlockIcon from './RlBlockIcon.vue'
import { INLINE_COMMANDS, BLOCK_COMMANDS } from './editorCommands'
import { TABLE_COMMANDS } from './tableCommands'

const meta: Meta<typeof RlEditorToolbar> = {
  title: 'Editor/Editor toolbar',
  component: RlEditorToolbar,
  parameters: { layout: 'padded' },
  args: {
    inlineCommands: INLINE_COMMANDS,
    blockCommands: BLOCK_COMMANDS,
    tableCommands: TABLE_COMMANDS,
  },
  decorators: [
    () => ({
      template: `
        <div style="border:1px solid var(--rl-color-border); border-radius:var(--rl-radius-md); overflow:hidden; max-width:620px">
          <story />
          <div style="padding:16px; color:var(--rl-color-text-subtle); font-size:var(--rl-font-size-sm)">
            The document would be here.
          </div>
        </div>
      `,
    }),
  ],
}
export default meta
type Story = StoryObj<typeof RlEditorToolbar>

/**
 * Presentational: it renders the buttons and emits `run`. Which command a
 * button fires and whether it looks active are both the parent's call, taken
 * from ProseMirror state, so nothing here reflects a real cursor.
 */
export const Playground: Story = {}

/** Bold and "heading 2" are on at the cursor, so both buttons read pressed. */
export const WithActiveCommands: Story = {
  args: { activeIds: new Set(['strong', 'h2']) },
}

/**
 * Unavailable buttons are `aria-disabled`, not natively `disabled`, so they
 * keep their place in the tab order: a cursor move that disables the button
 * you are tabbed to must not drop focus to `<body>` mid-interaction. Their
 * tooltip says "not available here" and the cursor stays default, because
 * `not-allowed` reads as an error rather than as "not in this context".
 */
export const WithUnavailableCommands: Story = {
  args: { unavailableIds: new Set(['inlineCode', 'codeBlock', 'blockquote']) },
}

/**
 * The table group appears only while the cursor is inside a table. Seven
 * permanently-dead buttons would be a lot of chrome to carry on every
 * paragraph, and unlike the block commands these mean nothing outside one.
 */
export const InsideATable: Story = {
  args: { showTableGroup: true },
}

/** Inline commands alone; the divider only appears between groups that exist. */
export const InlineOnly: Story = {
  args: { blockCommands: [], tableCommands: [] },
}

/**
 * An app's own buttons, given the same state the built-in ones render from,
 * so they can light up and grey out on the same terms. A slot button must
 * `preventDefault` on mousedown too, or pressing it pulls focus out of the
 * document and collapses the selection before the command runs.
 */
export const WithExtraButtons: Story = {
  args: { activeIds: new Set(['reference']) },
  render: (args) => ({
    components: { RlEditorToolbar, RlBlockIcon },
    setup: () => ({ args, log: ref<string[]>([]) }),
    template: `
      <RlEditorToolbar v-bind="args" @run="log.push($event.id)">
        <template #extra="{ activeIds }">
          <button
            type="button"
            title="Insert reference"
            aria-label="Insert reference"
            :aria-pressed="activeIds.has('reference')"
            style="display:inline-flex; align-items:center; justify-content:center; width:28px; height:28px; border:none; border-radius:var(--rl-radius-sm); background:transparent; color:var(--rl-color-text-muted); cursor:pointer"
            @mousedown.prevent
          >
            <RlBlockIcon name="reference" />
          </button>
        </template>
      </RlEditorToolbar>
    `,
  }),
}
