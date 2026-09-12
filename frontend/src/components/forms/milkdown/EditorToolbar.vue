<script setup lang="ts">
/**
 * The editor's formatting toolbar.
 *
 * Presentational, like the two menus: it renders the command buttons and their
 * active state and emits `run`. It holds no editor reference, so which command
 * a button fires and whether it is currently active are both decided by the
 * parent from ProseMirror state.
 *
 * Buttons preventDefault on mousedown so pressing one does not pull focus out
 * of the document. Without that the selection collapses before the command
 * runs, and formatting a selection from the toolbar becomes impossible.
 */
import BlockIcon from './BlockIcon.vue'
import type { EditorCommand } from './editorCommands'

const props = defineProps<{
  inlineCommands: readonly EditorCommand[]
  blockCommands: readonly EditorCommand[]
  /** Ids of the commands whose formatting is active at the cursor. */
  activeIds: Set<string>
  /** Ids of the commands that would do nothing here, so their buttons disable. */
  unavailableIds: Set<string>
  /** Row and column operations, shown only inside a table. */
  tableCommands: readonly EditorCommand[]
  /** Whether the cursor is in a table, which is when the group appears. */
  showTableGroup: boolean
}>()

const emit = defineEmits<{
  run: [command: EditorCommand]
  openEntityPicker: []
}>()

/**
 * Why a disabled button is `aria-disabled` and not natively `disabled`.
 *
 * A native `disabled` control cannot be focused, so if the cursor moves into a
 * context that disables the button the user is currently tabbed to, focus is
 * dropped to `<body>` mid-interaction. `aria-disabled` announces the state
 * while keeping the control in the tab order. Because it does not prevent
 * activation, the click handler and the editor's `runCommand` both refuse.
 */
function onActivate(cmd: EditorCommand): void {
  if (props.unavailableIds.has(cmd.id)) return
  emit('run', cmd)
}
</script>

<template>
  <div class="editor-toolbar" role="toolbar" aria-label="Formatting">
    <div class="toolbar-group">
      <button
        v-for="cmd in props.inlineCommands"
        :key="cmd.id"
        type="button"
        class="toolbar-button"
        :class="{
          'is-active': props.activeIds.has(cmd.id),
          'is-unavailable': props.unavailableIds.has(cmd.id),
        }"
        :title="props.unavailableIds.has(cmd.id) ? `${cmd.label} (not available here)` : cmd.label"
        :aria-label="cmd.label"
        :aria-pressed="props.activeIds.has(cmd.id)"
        :aria-disabled="props.unavailableIds.has(cmd.id)"
        @mousedown.prevent
        @click="onActivate(cmd)"
      >
        <BlockIcon :name="cmd.id" />
      </button>
    </div>

    <span class="toolbar-divider" role="separator" />

    <div class="toolbar-group">
      <button
        v-for="cmd in props.blockCommands"
        :key="cmd.id"
        type="button"
        class="toolbar-button"
        :class="{
          'is-active': props.activeIds.has(cmd.id),
          'is-unavailable': props.unavailableIds.has(cmd.id),
        }"
        :title="props.unavailableIds.has(cmd.id) ? `${cmd.label} (not available here)` : cmd.label"
        :aria-label="cmd.label"
        :aria-pressed="props.activeIds.has(cmd.id)"
        :aria-disabled="props.unavailableIds.has(cmd.id)"
        @mousedown.prevent
        @click="onActivate(cmd)"
      >
        <BlockIcon :name="cmd.id" />
      </button>
    </div>

    <!-- Only while the cursor is in a table. Seven permanently-disabled
         buttons would be a lot of dead chrome to carry on every paragraph,
         and unlike the block commands these have no meaning outside one. -->
    <template v-if="props.showTableGroup">
      <span class="toolbar-divider" role="separator" />

      <div class="toolbar-group" aria-label="Table">
        <button
          v-for="cmd in props.tableCommands"
          :key="cmd.id"
          type="button"
          class="toolbar-button"
          :class="{ 'is-unavailable': props.unavailableIds.has(cmd.id) }"
          :title="
            props.unavailableIds.has(cmd.id) ? `${cmd.label} (not available here)` : cmd.label
          "
          :aria-label="cmd.label"
          :aria-disabled="props.unavailableIds.has(cmd.id)"
          @mousedown.prevent
          @click="onActivate(cmd)"
        >
          <BlockIcon :name="cmd.id" />
        </button>
      </div>
    </template>

    <span class="toolbar-divider" role="separator" />

    <div class="toolbar-group">
      <!-- The connected-nodes glyph reads as "graph reference", which is what
           a backticked entity ID is: an edge into the project graph. -->
      <button
        type="button"
        class="toolbar-button entity-ref-button"
        title="Insert entity reference"
        aria-label="Insert entity reference"
        @mousedown.prevent
        @click="emit('openEntityPicker')"
      >
        <BlockIcon name="entityRef" />
      </button>
    </div>
  </div>
</template>
