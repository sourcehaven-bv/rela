<script setup lang="ts">
/**
 * Aside within a body of text: a tip, a caveat, a note.
 *
 * Quieter than a banner and never announced, because it is part of the
 * reading order rather than something that has just happened.
 */
import RlIcon from '../common/RlIcon.vue'
import RlText from '../common/RlText.vue'
import type { MessageTone } from './types'

withDefaults(defineProps<{ tone?: MessageTone; title?: string }>(), { tone: 'info' })

const icons = {
  info: 'info',
  success: 'check',
  warning: 'warning',
  danger: 'alert',
} as const
</script>

<template>
  <aside class="rl-callout" :class="`rl-callout--${tone}`">
    <RlIcon :name="icons[tone]" :size="16" class="rl-callout__icon" aria-hidden="true" />
    <div class="rl-callout__content">
      <RlText v-if="title" size="sm" weight="semibold">{{ title }}</RlText>
      <RlText as="p" size="sm" class="rl-callout__body"><slot /></RlText>
    </div>
  </aside>
</template>

<style scoped>
.rl-callout {
  display: flex;
  gap: var(--rl-space-2);
  padding: var(--rl-space-3);
  border-left: 3px solid currentColor;
  border-radius: 0 var(--rl-radius-sm) var(--rl-radius-sm) 0;
  background: var(--rl-color-bg-sunken);
}

.rl-callout__icon { flex: none; margin-top: 2px; }

.rl-callout__content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  /* The text stays neutral; only the rule and icon carry the tone. */
  color: var(--rl-color-text);
}

.rl-callout__body { margin: 0; }

.rl-callout--info { color: var(--rl-color-info-fg); }
.rl-callout--success { color: var(--rl-color-success-fg); }
.rl-callout--warning { color: var(--rl-color-warning-fg); }
.rl-callout--danger { color: var(--rl-color-danger); }
</style>
