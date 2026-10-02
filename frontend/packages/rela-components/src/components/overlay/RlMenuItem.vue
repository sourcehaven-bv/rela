<script setup lang="ts">
/**
 * One row in a menu.
 *
 * A button by default, never a div with a click handler, so it is reachable by
 * keyboard and announced as an action. `tone="danger"` marks a destructive
 * entry such as Delete.
 *
 * Pass `href` for a row that navigates. That makes it an `<a>`, which is not
 * cosmetic: a real link can be opened in a new tab, copied, and is announced
 * as going somewhere rather than doing something. A menu whose entries are
 * places — a workspace, a space, a recent document — wants links.
 */
import { computed } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import type { IconName } from '../common/icons'

const props = withDefaults(
  defineProps<{
    icon?: IconName
    tone?: 'default' | 'danger'
    disabled?: boolean
    /** Right-aligned hint, usually the keyboard shortcut. */
    shortcut?: string
    /** Where this row goes. Present makes the row a link rather than a button. */
    href?: string
    /**
     * Whether this row is the one already in effect: the current space, the
     * active sort, the view being shown.
     *
     * Draws a check in the trailing column and reports `aria-current`, so the
     * state is not carried by the tick alone. The row stays pressable: a menu
     * that disables its current entry gives a user who opened the menu by
     * mistake nothing to confirm, and re-selecting where you already are is
     * harmless.
     *
     * The check sits where `shortcut` would, since a row that is already in
     * effect is not one you reach for by accelerator.
     */
    current?: boolean
  }>(),
  { tone: 'default', disabled: false, current: false },
)

defineEmits<{ click: [event: MouseEvent] }>()

const tag = computed(() => (props.href && !props.disabled ? 'a' : 'button'))
</script>

<template>
  <component
    :is="tag"
    :type="tag === 'button' ? 'button' : undefined"
    :href="tag === 'a' ? href : undefined"
    role="menuitem"
    class="rl-menu-item"
    :class="[`rl-menu-item--${tone}`, { 'rl-menu-item--current': current }]"
    :disabled="tag === 'button' && disabled ? true : undefined"
    :aria-disabled="tag === 'a' && disabled ? 'true' : undefined"
    :aria-current="current ? (tag === 'a' ? 'page' : 'true') : undefined"
    @click="$emit('click', $event)"
  >
    <RlIcon v-if="icon" :name="icon" :size="15" class="rl-menu-item__icon" aria-hidden="true" />
    <span class="rl-menu-item__label"><slot /></span>
    <!--
      The check is decorative: `aria-current` above is what conveys this, so a
      screen reader does not hear a stray "check" glyph in the row.
    -->
    <RlIcon
      v-if="current"
      name="check"
      :size="15"
      class="rl-menu-item__check"
      aria-hidden="true"
    />
    <!-- Decorative: the shortcut is listed in the shortcuts sheet. -->
    <span
      v-else-if="shortcut"
      class="rl-menu-item__shortcut"
      aria-hidden="true"
      >{{ shortcut }}</span
    >
  </component>
</template>

<style scoped>
.rl-menu-item {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  width: 100%;
  padding: var(--rl-space-2);
  border: none;
  border-radius: var(--rl-radius-sm);
  background: transparent;
  color: var(--rl-color-text);
  font-family: inherit;
  font-size: var(--rl-font-size-md);
  text-align: left;
  cursor: pointer;
}

/* The link variant is the same row; it only has to shed the anchor defaults. */
.rl-menu-item { text-decoration: none; }

.rl-menu-item:hover:not(:disabled):not([aria-disabled='true']) {
  background: var(--rl-color-bg-hover);
}

.rl-menu-item:disabled,
.rl-menu-item[aria-disabled='true'] { opacity: 0.5; cursor: not-allowed; }

.rl-menu-item__icon { flex: none; color: var(--rl-color-text-muted); }
.rl-menu-item__label { flex: 1; min-width: 0; }

/*
 * The current row carries its weight, not a fill: the menu's hover and focus
 * states are both fills, and a third would make the row that is merely under
 * the cursor look like the row that is in effect.
 */
.rl-menu-item--current .rl-menu-item__label { font-weight: var(--rl-font-weight-medium); }
.rl-menu-item__check { flex: none; color: var(--rl-color-accent); }

.rl-menu-item__shortcut {
  flex: none;
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-subtle);
}

.rl-menu-item--danger { color: var(--rl-color-danger); }
.rl-menu-item--danger .rl-menu-item__icon { color: var(--rl-color-danger); }
.rl-menu-item--danger:hover:not(:disabled) { background: var(--rl-color-danger-bg); }

.rl-menu-item:focus-visible {
  outline: none;
  background: var(--rl-color-bg-hover);
  box-shadow: inset 0 0 0 var(--rl-focus-ring-width) var(--rl-color-focus);
}

.rl-menu-item--danger:focus-visible { box-shadow: inset 0 0 0 var(--rl-focus-ring-width) var(--rl-color-danger); }

@media (pointer: coarse) {
  .rl-menu-item { min-height: var(--rl-tap-target); }
}
</style>
