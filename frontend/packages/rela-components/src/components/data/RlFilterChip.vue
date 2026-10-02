<script setup lang="ts">
/**
 * One active filter, shown as a removable chip.
 *
 * The remove control is a separate button rather than the whole chip being
 * clickable, so a chip can also be pressed to edit the filter without the two
 * actions being ambiguous. Its label names the filter, since "Remove" alone
 * means nothing in a row of six chips.
 */
import RlIcon from '../common/RlIcon.vue'
import type { IconName } from '../common/icons'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

withDefaults(
  defineProps<{
    label: string
    /** The chosen value, shown after the label. */
    value?: string
    icon?: IconName
    removable?: boolean
    disabled?: boolean
  }>(),
  { removable: true, disabled: false },
)

defineEmits<{ remove: []; click: [] }>()
</script>

<template>
  <span class="rl-filter-chip" :class="{ 'rl-filter-chip--disabled': disabled }">
    <RlIcon v-if="icon" :name="icon" :size="13" aria-hidden="true" />

    <span class="rl-filter-chip__label">
      {{ label }}<template v-if="value">: <b>{{ value }}</b></template>
    </span>

    <button
      v-if="removable"
      type="button"
      class="rl-filter-chip__remove"
      :disabled="disabled"
      :aria-label="messages.removeFilter({ label, value })"
      @click="$emit('remove')"
    >
      <RlIcon name="x" :size="12" aria-hidden="true" />
    </button>
  </span>
</template>

<style scoped>
.rl-filter-chip {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  padding: 2px var(--rl-space-1) 2px var(--rl-space-2);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-pill);
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text);
  font-size: var(--rl-font-size-sm);
  line-height: var(--rl-line-height-tight);
}

.rl-filter-chip--disabled { opacity: 0.5; }

.rl-filter-chip__label b { font-weight: var(--rl-font-weight-semibold); }

.rl-filter-chip__remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--rl-color-text-muted);
  cursor: pointer;
}

.rl-filter-chip__remove:hover:not(:disabled) {
  background: var(--rl-color-bg-active);
  color: var(--rl-color-text);
}

.rl-filter-chip__remove:focus-visible {
  outline: var(--rl-focus-ring-width) solid var(--rl-color-focus);
  outline-offset: 1px;
}
</style>
