<script setup lang="ts">
/**
 * One row in a slide-out panel's list: an optional checkbox, a label that
 * opens the next panel, and trailing meta such as a time.
 *
 * The row is a button rather than a link by default, because in a stack the
 * click usually opens the panel beside it rather than navigating. Pass `as`
 * with a router link for a list that really does change the URL, the same
 * way `NavItem.as` does and for the same reason: carried as the component
 * itself, so this library stays free of a router.
 *
 * The checkbox is a real control beside the button, not inside it. Nesting
 * it would make one click ambiguous, and ticking a task off a list is not
 * the same action as opening it.
 */
import { computed, type Component } from 'vue'

const props = withDefaults(
  defineProps<{
    label: string
    /** Trailing text, such as a time or a day. */
    meta?: string
    /** Draws the meta in the accent colour, for something due or overdue. */
    metaHighlight?: boolean
    /** Shows a checkbox. Bind `checked` to make it do something. */
    checkable?: boolean
    checked?: boolean
    /** Marks the row whose panel is open beside it. */
    selected?: boolean
    /** What the label renders as. See `NavItem.as`. */
    as?: 'button' | 'a' | Component
    /** Extra attributes for the rendered label, such as `href` or `to`. */
    attrs?: Record<string, unknown>
  }>(),
  {
    checkable: false,
    checked: false,
    selected: false,
    metaHighlight: false,
    as: 'button',
    attrs: undefined,
  },
)

const emit = defineEmits<{
  select: []
  'update:checked': [value: boolean]
}>()

const isButton = computed(() => props.as === 'button')
</script>

<template>
  <div class="rl-panel-list-row" :class="{ 'rl-panel-list-row--selected': selected }">
    <input
      v-if="checkable"
      type="checkbox"
      class="rl-panel-list-row__check"
      :checked="checked"
      :aria-label="label"
      @change="emit('update:checked', ($event.target as HTMLInputElement).checked)"
    />

    <component
      :is="as"
      v-bind="attrs"
      :type="isButton ? 'button' : undefined"
      :aria-current="selected ? 'true' : undefined"
      class="rl-panel-list-row__label"
      @click="emit('select')"
    >
      {{ label }}
    </component>

    <span
      v-if="meta"
      class="rl-panel-list-row__meta"
      :class="{ 'rl-panel-list-row__meta--highlight': metaHighlight }"
    >
      {{ meta }}
    </span>
  </div>
</template>

<style scoped>
.rl-panel-list-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  padding: var(--rl-space-3) var(--rl-space-5);
  border-bottom: 1px solid var(--rl-color-border);
}

.rl-panel-list-row:hover { background: var(--rl-color-bg-hover); }

.rl-panel-list-row--selected { background: var(--rl-color-bg-selected); }

/*
 * The ring is drawn by the row, so it surrounds the whole target rather than
 * the text inside it, and is inset so a row at the edge of the panel does
 * not have half its ring clipped away.
 */
.rl-panel-list-row:has(.rl-panel-list-row__label:focus-visible) {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

.rl-panel-list-row__check {
  flex: none;
  width: 16px;
  height: 16px;
  margin: 0;
  accent-color: var(--rl-color-accent);
  cursor: pointer;
  /*
   * Above the label's own hit area, which covers the row; without this the
   * checkbox would be under it and a click on the box would open the panel.
   */
  position: relative;
  z-index: 1;
}

.rl-panel-list-row__label {
  flex: 1;
  min-width: 0;
  padding: 0;
  border: none;
  background: none;
  font: inherit;
  color: inherit;
  text-align: left;
  text-decoration: none;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* The whole row opens the panel, not just the words in it. */
.rl-panel-list-row__label::after {
  content: '';
  position: absolute;
  inset: 0;
}

.rl-panel-list-row__label:focus-visible { outline: none; }

.rl-panel-list-row__meta {
  flex: none;
  position: relative;
  z-index: 1;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-subtle);
}

.rl-panel-list-row__meta--highlight { color: var(--rl-color-accent); }

@media (pointer: coarse) {
  .rl-panel-list-row { padding-block: var(--rl-space-4); }
}
</style>
