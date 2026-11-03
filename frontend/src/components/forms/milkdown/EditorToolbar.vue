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
import { useI18n } from 'vue-i18n'
import BlockIcon from './BlockIcon.vue'
import type { EditorCommand } from './editorCommands'

const { t } = useI18n()

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
  /** Undo and redo, in their own group at the end. */
  historyCommands: readonly EditorCommand[]
  /**
   * Whether the cursor is in a link, which reveals the unlink button.
   *
   * The floating panel is not a keyboard route: it sits outside the editable
   * div, so reaching its buttons means tabbing out of the editor entirely, and
   * it vanishes as soon as the caret moves. The toolbar carries the whole
   * feature instead — with the caret in a link, the Link button opens the
   * dialog prefilled (retarget) and this one removes it, so insert, retarget
   * and remove are all reachable without a pointer.
   */
  showUnlink: boolean
}>()

const emit = defineEmits<{
  run: [command: EditorCommand]
  openEntityPicker: []
  unlink: []
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
/**
 * The translated label per command id. The shared command modules keep their
 * English `label`, because `<rela-editor>` imports them and carries no
 * vue-i18n; a command missing here falls back to that English label.
 */
const COMMAND_LABELS: Record<string, () => string> = {
  strong: () => t('editor.command.bold'),
  emphasis: () => t('editor.command.italic'),
  strikethrough: () => t('editor.command.strikethrough'),
  inlineCode: () => t('editor.command.inlineCode'),
  link: () => t('editor.command.link'),
  h1: () => t('editor.command.heading1'),
  h2: () => t('editor.command.heading2'),
  h3: () => t('editor.command.heading3'),
  bulletList: () => t('editor.command.bulletList'),
  orderedList: () => t('editor.command.numberedList'),
  taskList: () => t('editor.command.taskList'),
  blockquote: () => t('editor.command.quote'),
  codeBlock: () => t('editor.command.codeBlock'),
  table: () => t('editor.command.table'),
  hr: () => t('editor.command.divider'),
  undo: () => t('editor.command.undo'),
  redo: () => t('editor.command.redo'),
  addRowBefore: () => t('editor.table.insertRowAbove'),
  addRowAfter: () => t('editor.table.insertRowBelow'),
  addColBefore: () => t('editor.table.insertColumnLeft'),
  addColAfter: () => t('editor.table.insertColumnRight'),
  deleteRow: () => t('editor.table.deleteRow'),
  deleteColumn: () => t('editor.table.deleteColumn'),
  deleteTable: () => t('editor.table.deleteTable'),
}

function label(cmd: EditorCommand): string {
  return COMMAND_LABELS[cmd.id]?.() ?? cmd.label
}

/** The tooltip: the label, plus why the button does nothing here. */
function tooltip(cmd: EditorCommand): string {
  return props.unavailableIds.has(cmd.id)
    ? t('editor.toolbar.notAvailable', { label: label(cmd) })
    : label(cmd)
}

function onActivate(cmd: EditorCommand): void {
  if (props.unavailableIds.has(cmd.id)) return
  emit('run', cmd)
}
</script>

<template>
  <div class="editor-toolbar" role="toolbar" :aria-label="t('editor.toolbar.formatting')">
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
        :title="tooltip(cmd)"
        :aria-label="label(cmd)"
        :aria-pressed="props.activeIds.has(cmd.id)"
        :aria-disabled="props.unavailableIds.has(cmd.id)"
        @mousedown.prevent
        @click="onActivate(cmd)"
      >
        <BlockIcon :name="cmd.id" />
      </button>

      <!-- Only while the cursor is in a link. Carrying a permanently-dead
           unlink button on every paragraph would be noise, and unlike the
           block commands it has no meaning outside one. -->
      <button
        v-if="props.showUnlink"
        type="button"
        class="toolbar-button"
        :title="t('editor.toolbar.removeLink')"
        :aria-label="t('editor.toolbar.removeLink')"
        @mousedown.prevent
        @click="emit('unlink')"
      >
        <BlockIcon name="unlink" />
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
        :title="tooltip(cmd)"
        :aria-label="label(cmd)"
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

      <div class="toolbar-group" :aria-label="t('editor.command.table')">
        <button
          v-for="cmd in props.tableCommands"
          :key="cmd.id"
          type="button"
          class="toolbar-button"
          :class="{ 'is-unavailable': props.unavailableIds.has(cmd.id) }"
          :title="tooltip(cmd)"
          :aria-label="label(cmd)"
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
        :title="t('editor.toolbar.insertEntityRef')"
        :aria-label="t('editor.toolbar.insertEntityRef')"
        @mousedown.prevent
        @click="emit('openEntityPicker')"
      >
        <BlockIcon name="entityRef" />
      </button>
    </div>

    <span class="toolbar-divider" role="separator" />

    <!-- History last: it acts on the document as a whole rather than on what
         is under the cursor, so it does not belong among the formatting
         groups. These are `aria-disabled` like every other button here, which
         matters most for these two — their availability flips on every
         transaction, so native `disabled` would drop focus mid-typing. -->
    <div class="toolbar-group" :aria-label="t('editor.toolbar.history')">
      <button
        v-for="cmd in props.historyCommands"
        :key="cmd.id"
        type="button"
        class="toolbar-button"
        :class="{ 'is-unavailable': props.unavailableIds.has(cmd.id) }"
        :title="tooltip(cmd)"
        :aria-label="label(cmd)"
        :aria-disabled="props.unavailableIds.has(cmd.id)"
        @mousedown.prevent
        @click="onActivate(cmd)"
      >
        <BlockIcon :name="cmd.id" />
      </button>
    </div>
  </div>
</template>
