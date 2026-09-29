<script setup lang="ts">
/**
 * The markdown editor's formatting toolbar.
 *
 * Presentational: it renders the command buttons and their state and emits
 * `run`. It holds no editor reference, so which command a button fires and
 * whether it is currently active are both decided by the parent from
 * ProseMirror state.
 *
 * Buttons preventDefault on mousedown so pressing one does not pull focus out
 * of the document. Without that the selection collapses before the command
 * runs, and formatting a selection from the toolbar becomes impossible. An
 * app adding its own buttons through `#extra` must do the same.
 */
import RlBlockIcon from './RlBlockIcon.vue'
import type { EditorCommand } from './editorCommands'

const props = withDefaults(
  defineProps<{
    inlineCommands?: readonly EditorCommand[]
    blockCommands?: readonly EditorCommand[]
    /** Ids of the commands whose formatting is active at the cursor. */
    activeIds?: ReadonlySet<string>
    /** Ids of the commands that would do nothing here, so their buttons disable. */
    unavailableIds?: ReadonlySet<string>
    /** Row and column operations, shown only inside a table. */
    tableCommands?: readonly EditorCommand[]
    /** Whether the cursor is in a table, which is when the group appears. */
    showTableGroup?: boolean
  }>(),
  {
    inlineCommands: () => [],
    blockCommands: () => [],
    activeIds: () => new Set<string>(),
    unavailableIds: () => new Set<string>(),
    tableCommands: () => [],
    showTableGroup: false,
  },
)

const emit = defineEmits<{ run: [command: EditorCommand] }>()

/**
 * Why a disabled button is `aria-disabled` and not natively `disabled`.
 *
 * A native `disabled` control cannot be focused, so if the cursor moves into a
 * context that disables the button the user is currently tabbed to, focus is
 * dropped to `<body>` mid-interaction. `aria-disabled` announces the state
 * while keeping the control in the tab order. Because it does not prevent
 * activation, both this handler and the editor's `runCommand` refuse.
 */
function onActivate(cmd: EditorCommand): void {
  if (props.unavailableIds.has(cmd.id)) return
  emit('run', cmd)
}

function titleFor(cmd: EditorCommand): string {
  return props.unavailableIds.has(cmd.id) ? `${cmd.label} (not available here)` : cmd.label
}
</script>

<template>
  <div class="rl-editor-toolbar" role="toolbar" aria-label="Formatting">
    <div v-if="props.inlineCommands.length" class="rl-editor-toolbar__group">
      <button
        v-for="cmd in props.inlineCommands"
        :key="cmd.id"
        type="button"
        class="rl-editor-toolbar__button"
        :class="{
          'is-active': props.activeIds.has(cmd.id),
          'is-unavailable': props.unavailableIds.has(cmd.id),
        }"
        :title="titleFor(cmd)"
        :aria-label="cmd.label"
        :aria-pressed="props.activeIds.has(cmd.id)"
        :aria-disabled="props.unavailableIds.has(cmd.id)"
        @mousedown.prevent
        @click="onActivate(cmd)"
      >
        <RlBlockIcon :name="cmd.id" />
      </button>
    </div>

    <span
      v-if="props.inlineCommands.length && props.blockCommands.length"
      class="rl-editor-toolbar__divider"
      role="separator"
    />

    <div v-if="props.blockCommands.length" class="rl-editor-toolbar__group">
      <button
        v-for="cmd in props.blockCommands"
        :key="cmd.id"
        type="button"
        class="rl-editor-toolbar__button"
        :class="{
          'is-active': props.activeIds.has(cmd.id),
          'is-unavailable': props.unavailableIds.has(cmd.id),
        }"
        :title="titleFor(cmd)"
        :aria-label="cmd.label"
        :aria-pressed="props.activeIds.has(cmd.id)"
        :aria-disabled="props.unavailableIds.has(cmd.id)"
        @mousedown.prevent
        @click="onActivate(cmd)"
      >
        <RlBlockIcon :name="cmd.id" />
      </button>
    </div>

    <!-- Only while the cursor is in a table. Seven permanently-disabled
         buttons would be a lot of dead chrome to carry on every paragraph,
         and unlike the block commands these have no meaning outside one. -->
    <template v-if="props.showTableGroup && props.tableCommands.length">
      <span class="rl-editor-toolbar__divider" role="separator" />

      <div class="rl-editor-toolbar__group" aria-label="Table">
        <button
          v-for="cmd in props.tableCommands"
          :key="cmd.id"
          type="button"
          class="rl-editor-toolbar__button"
          :class="{ 'is-unavailable': props.unavailableIds.has(cmd.id) }"
          :title="titleFor(cmd)"
          :aria-label="cmd.label"
          :aria-disabled="props.unavailableIds.has(cmd.id)"
          @mousedown.prevent
          @click="onActivate(cmd)"
        >
          <RlBlockIcon :name="cmd.id" />
        </button>
      </div>
    </template>

    <!-- App-supplied buttons: a mention picker, a reference inserter, whatever
         the host adds on top of the generic command set. Given the same state
         the built-in buttons render from, so an app button can light up and
         grey out on the same terms. -->
    <template v-if="$slots.extra">
      <span class="rl-editor-toolbar__divider" role="separator" />
      <div class="rl-editor-toolbar__group">
        <slot
          name="extra"
          :active-ids="props.activeIds"
          :unavailable-ids="props.unavailableIds"
        />
      </div>
    </template>
  </div>
</template>

<style scoped>
.rl-editor-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-1);
  padding: var(--rl-space-1) var(--rl-space-2);
  border-bottom: 1px solid var(--rl-color-border);
  background: var(--rl-color-bg);
}

.rl-editor-toolbar__group {
  display: flex;
  align-items: center;
  gap: 2px;
}

.rl-editor-toolbar__divider {
  width: 1px;
  align-self: stretch;
  margin: var(--rl-space-1) var(--rl-space-1);
  background: var(--rl-color-border);
}

.rl-editor-toolbar__button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: var(--rl-radius-sm);
  background: transparent;
  color: var(--rl-color-text-muted);
  cursor: pointer;
}

.rl-editor-toolbar__button:hover {
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text);
}

.rl-editor-toolbar__button:focus-visible {
  outline: var(--rl-focus-ring-width) solid var(--rl-focus-ring-color);
  outline-offset: 1px;
}

.rl-editor-toolbar__button.is-active {
  background: var(--rl-color-bg-selected);
  color: var(--rl-color-accent);
}

/* `aria-disabled`, not `disabled`, so the control keeps its place in the tab
   order; see `onActivate`. The cursor stays default rather than
   `not-allowed`, which reads as an error rather than as "not here". */
.rl-editor-toolbar__button.is-unavailable {
  color: var(--rl-color-text-subtle);
  opacity: 0.5;
  cursor: default;
}

.rl-editor-toolbar__button.is-unavailable:hover {
  background: transparent;
  color: var(--rl-color-text-subtle);
}
</style>
