<script setup lang="ts">
/**
 * Small group of mutually exclusive choices, such as Month / Week.
 *
 * Distinct from RlViewTabs: tabs switch between panels of content and use the
 * tab role, while this changes a setting that reshapes the view already on
 * screen. It is a radio group, which is what it behaves like.
 *
 * Arrow keys move between options and the roving tabindex keeps the group a
 * single Tab stop, as the radio group pattern requires. When no option is
 * checked, the first enabled one is the Tab stop, so the group stays
 * reachable.
 *
 * ## Options that are links
 *
 * Where each choice is a place with its own URL (a world, a face of a
 * record), give the options `as` and `attrs` and they render as links, as in
 * `RlViewTabs`. The group is then a set of links, not a radio group: each
 * link is its own Tab stop, the current one carries `aria-current`, and the
 * arrow keys do nothing. Arrow-key selection would navigate on every key
 * press, and only an anchor gives the middle click and the modifier click.
 * Pass all options as links or none.
 */
import { computed, ref, type Component } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import type { IconName } from '../common/icons'

export interface SegmentOption {
  value: string
  label: string
  icon?: IconName
  disabled?: boolean
  /**
   * Extra text for a screen reader only, read after the label. With
   * iconOnly it is part of the button's aria-label.
   */
  srHint?: string
  /** What the option renders as; a link for a choice with its own URL. */
  as?: 'button' | 'a' | Component
  /** Extra attributes for the rendered element, such as `to` or `href`. */
  attrs?: Record<string, unknown>
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: SegmentOption[]
    label: string
    size?: 'sm' | 'md'
    /** Shows only icons, with the label as the accessible name. */
    iconOnly?: boolean
  }>(),
  { size: 'md', iconOnly: false },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const root = ref<HTMLElement | null>(null)

const linked = computed(() => props.options.some((o) => o.as && o.as !== 'button'))

/** The option that takes Tab: the checked one, else the first enabled one. */
const tabStop = computed(() => {
  if (props.options.some((o) => o.value === props.modelValue && !o.disabled)) return props.modelValue
  return props.options.find((o) => !o.disabled)?.value
})

function move(direction: 1 | -1) {
  const usable = props.options.filter((option) => !option.disabled)
  // With nothing checked, move from the tab stop, which holds the focus.
  const current = usable.findIndex((option) => option.value === tabStop.value)
  const next = usable[(current + direction + usable.length) % usable.length]
  if (!next) return

  emit('update:modelValue', next.value)
  // Focus follows selection, which is the expected behaviour for a radio group.
  root.value?.querySelector<HTMLElement>(`[data-value="${next.value}"]`)?.focus()
}
</script>

<template>
  <div
    v-if="linked"
    class="rl-segmented"
    :class="`rl-segmented--${size}`"
    role="group"
    :aria-label="label"
  >
    <template v-for="option in options" :key="option.value">
      <span
        v-if="option.disabled"
        class="rl-segmented__option rl-segmented__option--link rl-segmented__option--disabled"
        :class="{ 'rl-segmented__option--active': option.value === modelValue }"
        :data-value="option.value"
        role="link"
        aria-disabled="true"
        :title="option.label"
        :aria-current="option.value === modelValue ? 'true' : undefined"
      >
        <RlIcon v-if="option.icon" :name="option.icon" :size="size === 'sm' ? 14 : 15" aria-hidden="true" />
        <span class="rl-segmented__text">{{ option.label }}</span>
        <span v-if="option.srHint" class="rl-visually-hidden">{{ option.srHint }}</span>
      </span>
      <component
        :is="option.as && option.as !== 'button' ? option.as : 'a'"
        v-else
        v-bind="option.attrs"
        class="rl-segmented__option rl-segmented__option--link"
        :class="{ 'rl-segmented__option--active': option.value === modelValue }"
        :data-value="option.value"
        :title="option.label"
        :aria-current="option.value === modelValue ? 'true' : undefined"
        @click="emit('update:modelValue', option.value)"
      >
        <RlIcon v-if="option.icon" :name="option.icon" :size="size === 'sm' ? 14 : 15" aria-hidden="true" />
        <span class="rl-segmented__text">{{ option.label }}</span>
        <span v-if="option.srHint" class="rl-visually-hidden">{{ option.srHint }}</span>
      </component>
    </template>
  </div>
  <div
    v-else
    ref="root"
    class="rl-segmented"
    :class="`rl-segmented--${size}`"
    role="radiogroup"
    :aria-label="label"
    @keydown.left.prevent="move(-1)"
    @keydown.up.prevent="move(-1)"
    @keydown.right.prevent="move(1)"
    @keydown.down.prevent="move(1)"
  >
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      role="radio"
      class="rl-segmented__option"
      :class="{ 'rl-segmented__option--active': option.value === modelValue }"
      :data-value="option.value"
      :aria-checked="option.value === modelValue"
      :aria-label="iconOnly ? `${option.label}${option.srHint ?? ''}` : undefined"
      :disabled="option.disabled"
      :tabindex="option.value === tabStop ? 0 : -1"
      @click="emit('update:modelValue', option.value)"
    >
      <RlIcon v-if="option.icon" :name="option.icon" :size="size === 'sm' ? 14 : 15" aria-hidden="true" />
      <span v-if="!iconOnly">{{ option.label }}</span>
      <span v-if="option.srHint && !iconOnly" class="rl-visually-hidden">{{ option.srHint }}</span>
    </button>
  </div>
</template>

<style scoped>
.rl-segmented {
  display: inline-flex;
  gap: 2px;
  padding: 2px;
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg-hover);
}

.rl-segmented__option {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  border: none;
  border-radius: var(--rl-radius-sm);
  background: transparent;
  color: var(--rl-color-text-muted);
  font-family: inherit;
  font-weight: var(--rl-font-weight-medium);
  cursor: pointer;
  text-decoration: none;
  transition: background-color var(--rl-duration-fast) var(--rl-ease), color var(--rl-duration-fast) var(--rl-ease);
}

.rl-segmented--sm .rl-segmented__option {
  height: 24px;
  padding: 0 var(--rl-space-2);
  font-size: var(--rl-font-size-xs);
}
.rl-segmented--md .rl-segmented__option {
  height: 28px;
  padding: 0 var(--rl-space-3);
  font-size: var(--rl-font-size-sm);
}

.rl-segmented__option:hover:not(:disabled):not(.rl-segmented__option--disabled):not(.rl-segmented__option--active) {
  color: var(--rl-color-text);
}

/*
 * The active option is raised off the track, not just recoloured. It takes
 * the chip token rather than the page background because it sits on the
 * track: the two have to be told apart from each other, and in the dark theme
 * the page is darker than the track, so the page colour sank the chip instead
 * of lifting it.
 */
.rl-segmented__option--active {
  background: var(--rl-color-chip-raised);
  color: var(--rl-color-text);
  box-shadow: var(--rl-shadow-sm);
}

/* Link mode: labels are operator text of any length, so they stay on one
   line and truncate. */
.rl-segmented__option--link {
  min-width: 0;
}
.rl-segmented__text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-segmented__option:disabled,
.rl-segmented__option--disabled { opacity: 0.5; cursor: not-allowed; }

.rl-segmented__option:focus-visible {
  outline: var(--rl-focus-ring-width) solid var(--rl-color-focus);
  outline-offset: 1px;
}

@media (pointer: coarse) {
  .rl-segmented--sm .rl-segmented__option,
  .rl-segmented--md .rl-segmented__option { min-height: 32px; }
}
</style>
