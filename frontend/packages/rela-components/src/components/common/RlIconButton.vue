<script setup lang="ts">
/** Square, icon-only button. Used across toolbars, headers and the sidebar. */
import RlIcon from './RlIcon.vue'
import type { IconName } from './icons'

withDefaults(
  defineProps<{
    icon: IconName
    label: string
    size?: number
    active?: boolean
    pressed?: boolean
    /** `danger` marks a destructive action, such as delete. */
    tone?: 'default' | 'danger'
  }>(),
  { size: 18, active: false, tone: 'default' },
)

defineEmits<{ click: [event: MouseEvent] }>()
</script>

<template>
  <button
    type="button"
    class="rl-icon-button"
    :class="[`rl-icon-button--tone-${tone}`, { 'rl-icon-button--active': active }]"
    :aria-label="label"
    :aria-pressed="pressed"
    @click="$emit('click', $event)"
  >
    <RlIcon :name="icon" :size="size" />
  </button>
</template>

<style scoped>
.rl-icon-button {
  display: flex;
  padding: var(--rl-space-1);
  border: none;
  border-radius: var(--rl-radius-sm);
  background: transparent;
  color: var(--rl-color-text-subtle);
  cursor: pointer;
}

.rl-icon-button:hover {
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text-muted);
}

/* Destructive actions read as such on hover rather than at rest, so a
 * toolbar of icons stays calm until the pointer reaches the delete. */
.rl-icon-button--tone-danger:hover {
  background: var(--rl-color-danger-bg);
  color: var(--rl-color-danger);
}
.rl-icon-button--tone-danger:focus-visible { outline-color: var(--rl-color-danger); }

/* Starred / toggled-on state: fill the glyph rather than just tint it. */
.rl-icon-button--active { color: var(--rl-color-starred); }
.rl-icon-button--active :deep(.rl-icon) { fill: currentColor; }

.rl-icon-button:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}

/*
 * Touch devices need a comfortable hit area (WCAG 2.5.8 sets 24px as the
 * floor). The pseudo-element grows the target without changing the visual
 * size of the control.
 */
@media (pointer: coarse) {
  .rl-icon-button {
    position: relative;
  }

  .rl-icon-button::before {
    content: '';
    position: absolute;
    top: 50%;
    left: 50%;
    width: var(--rl-tap-target);
    height: var(--rl-tap-target);
    transform: translate(-50%, -50%);
  }
}
</style>
