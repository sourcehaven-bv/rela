<script setup lang="ts">
/**
 * Inline message that stays until the condition behind it changes, such as a
 * permissions notice or a degraded-service warning.
 *
 * Distinct from a toast, which is transient and interrupts. A banner sits in
 * the layout and is read in place.
 */
import { computed } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import RlIconButton from '../common/RlIconButton.vue'
import RlText from '../common/RlText.vue'
import type { MessageTone } from './types'

const props = withDefaults(
  defineProps<{
    tone?: MessageTone
    title?: string
    dismissible?: boolean
  }>(),
  { tone: 'info', dismissible: false },
)

defineEmits<{ dismiss: [] }>()

const icons = {
  info: 'info',
  success: 'check',
  warning: 'warning',
  danger: 'alert',
} as const

/*
 * A warning or an error is announced as soon as it appears; information and
 * success are not, because interrupting to say something went right is noise.
 */
const live = computed(() =>
  props.tone === 'danger' || props.tone === 'warning' ? 'assertive' : undefined,
)
const role = computed(() => (props.tone === 'danger' ? 'alert' : 'status'))
</script>

<template>
  <div class="rl-banner" :class="`rl-banner--${tone}`" :role="role" :aria-live="live">
    <RlIcon :name="icons[tone]" :size="18" class="rl-banner__icon" aria-hidden="true" />

    <div class="rl-banner__content">
      <RlText v-if="title" size="md" weight="semibold" class="rl-banner__title">{{ title }}</RlText>
      <RlText v-if="$slots.default" as="p" size="sm" class="rl-banner__body"><slot /></RlText>
    </div>

    <div v-if="$slots.actions" class="rl-banner__actions"><slot name="actions" /></div>

    <RlIconButton
      v-if="dismissible"
      icon="x"
      label="Dismiss message"
      :size="14"
      class="rl-banner__close"
      @click="$emit('dismiss')"
    />
  </div>
</template>

<style scoped>
.rl-banner {
  display: flex;
  align-items: flex-start;
  gap: var(--rl-space-3);
  padding: var(--rl-space-3) var(--rl-space-4);
  border-radius: var(--rl-radius-md);
  /*
   * Colour is reinforced by the icon and the wording, so the meaning does not
   * rest on hue alone (WCAG 1.4.1).
   */
  border-left: 3px solid currentColor;
}

.rl-banner__icon { flex: none; margin-top: 1px; }

.rl-banner__content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.rl-banner__title { color: inherit; }
.rl-banner__body { margin: 0; color: inherit; }

.rl-banner__actions {
  flex: none;
  display: flex;
  gap: var(--rl-space-2);
  align-items: center;
}

.rl-banner__close { flex: none; color: inherit; }

.rl-banner--info { background: var(--rl-color-info-bg); color: var(--rl-color-info-fg); }
.rl-banner--success { background: var(--rl-color-success-bg); color: var(--rl-color-success-fg); }
.rl-banner--warning { background: var(--rl-color-warning-bg); color: var(--rl-color-warning-fg); }
.rl-banner--danger { background: var(--rl-color-danger-bg); color: var(--rl-color-danger); }

@media (max-width: 767px) {
  .rl-banner { flex-wrap: wrap; }
  .rl-banner__actions { width: 100%; }
}
</style>
