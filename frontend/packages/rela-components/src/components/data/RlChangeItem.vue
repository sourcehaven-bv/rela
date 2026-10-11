<script setup lang="ts">
/**
 * One change in an `RlChangeList`: a mark for its kind, what changed, and
 * optionally the value before and after.
 *
 * The kind is a mark and a word. The mark is what a sighted reader scans for;
 * the word is read before the label by a screen reader, because a colour or a
 * plus sign is not announced.
 *
 * `before` and `after` are slots as well as props, so a value can be drawn the
 * way it is drawn everywhere else: a status as a dot and a label, a person as
 * an avatar. Pass a string for plain text.
 *
 * A row that leads somewhere, such as back to the screen where the change was
 * made, takes `as`, the same way `RlPanelListRow` does: a button by name, or a
 * router link as the component itself, which keeps this library free of a
 * router. Without `as` the row is plain text.
 */
import { computed, type Component } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import type { IconName } from '../common/icons'
import { useMessages } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    kind: 'added' | 'changed' | 'removed'
    /** What changed, in the reader's words: `Field Due`, `Option XXL`. */
    label: string
    /** A short qualifier after the label: `Date`, `after Priority`. */
    detail?: string
    /** The value before the change. Shown struck through. */
    before?: string
    /** The value after the change. */
    after?: string
    /** Renders the row as a button, a link, or the given component. */
    as?: 'button' | 'a' | Component
    /** Extra attributes for the rendered row, such as `href` or `to`. */
    attrs?: Record<string, unknown>
  }>(),
  { detail: undefined, before: undefined, after: undefined, as: undefined, attrs: undefined },
)

defineEmits<{ select: [] }>()

const slots = defineSlots<{ before?: () => unknown; after?: () => unknown }>()

const messages = useMessages()

const icons: Record<typeof props.kind, IconName> = { added: 'plus', changed: 'edit', removed: 'minus' }

const hasBefore = computed(() => props.before !== undefined || Boolean(slots.before))
const hasAfter = computed(() => props.after !== undefined || Boolean(slots.after))
</script>

<template>
  <li class="rl-change-item" :class="`rl-change-item--${kind}`">
    <component
      :is="as ?? 'div'"
      class="rl-change-item__row"
      :class="{ 'rl-change-item__row--interactive': as }"
      :type="as === 'button' ? 'button' : undefined"
      v-bind="attrs"
      @click="as && $emit('select')"
    >
      <span class="rl-change-item__mark" aria-hidden="true">
        <RlIcon :name="icons[kind]" :size="12" />
      </span>
      <span class="rl-change-item__text">
        <span class="rl-visually-hidden">{{ messages.changeKind({ kind }) }}</span>
        <span class="rl-change-item__label">{{ label }}</span>
        <span v-if="detail" class="rl-change-item__detail">{{ detail }}</span>
      </span>
      <span v-if="hasBefore || hasAfter" class="rl-change-item__values">
        <s v-if="hasBefore" class="rl-change-item__before"><span class="rl-visually-hidden">{{ messages.changeBefore() }} </span><slot name="before">{{ before }}</slot></s>
        <RlIcon v-if="hasBefore && hasAfter" name="arrow-right" :size="12" aria-hidden="true" class="rl-change-item__arrow" />
        <span v-if="hasAfter" class="rl-change-item__after"><span class="rl-visually-hidden">{{ messages.changeAfter() }} </span><slot name="after">{{ after }}</slot></span>
      </span>
      <RlIcon v-if="as" name="chevron-right" :size="14" aria-hidden="true" class="rl-change-item__chevron" />
    </component>
  </li>
</template>

<style scoped>
.rl-change-item + .rl-change-item {
  border-top: 1px solid var(--rl-color-border);
}

.rl-change-item__row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  width: 100%;
  min-height: var(--rl-row-height);
  padding: var(--rl-space-2) var(--rl-space-3);
  border: none;
  background: var(--rl-color-bg);
  font: inherit;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text);
  text-align: left;
  text-decoration: none;
}

.rl-change-item__row--interactive {
  cursor: pointer;
  transition: background-color var(--rl-duration-fast) var(--rl-ease);
}

.rl-change-item__row--interactive:hover {
  background: var(--rl-color-bg-hover);
}

.rl-change-item__row--interactive:focus-visible {
  outline: var(--rl-focus-ring-width) solid var(--rl-color-focus);
  outline-offset: -2px;
}

.rl-change-item__mark {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: var(--rl-radius-sm);
}

.rl-change-item--added .rl-change-item__mark {
  background: var(--rl-color-success-bg);
  color: var(--rl-color-success-fg);
}
.rl-change-item--changed .rl-change-item__mark {
  background: var(--rl-color-info-bg);
  color: var(--rl-color-info-fg);
}
.rl-change-item--removed .rl-change-item__mark {
  background: var(--rl-color-danger-bg);
  color: var(--rl-color-danger);
}

.rl-change-item__text {
  display: flex;
  flex: 1;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0 var(--rl-space-2);
  min-width: 0;
}

.rl-change-item__label {
  font-weight: var(--rl-font-weight-medium);
}

.rl-change-item--removed .rl-change-item__label {
  color: var(--rl-color-text-muted);
}

.rl-change-item__detail {
  color: var(--rl-color-text-muted);
}

.rl-change-item__values {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: var(--rl-space-2);
  max-width: 50%;
}

.rl-change-item__before {
  color: var(--rl-color-text-subtle);
}

.rl-change-item__arrow,
.rl-change-item__chevron {
  flex: none;
  color: var(--rl-color-text-subtle);
}

@media (pointer: coarse) {
  .rl-change-item__row--interactive {
    min-height: var(--rl-tap-target);
  }
}
</style>
