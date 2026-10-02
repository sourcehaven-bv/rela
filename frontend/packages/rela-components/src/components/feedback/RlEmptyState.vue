<script setup lang="ts">
/**
 * What a list shows when it has nothing in it.
 *
 * Distinguishes two cases, because they need different words: nothing has
 * been created yet, which calls for an invitation to create the first one, or
 * a filter matched nothing, which calls for a way to widen it.
 */
import RlHeading from '../common/RlHeading.vue'
import RlIcon from '../common/RlIcon.vue'
import RlText from '../common/RlText.vue'
import type { IconName } from '../common/icons'

withDefaults(
  defineProps<{
    title: string
    description?: string
    icon?: IconName
    size?: 'sm' | 'md'
  }>(),
  { icon: 'inbox', size: 'md' },
)
</script>

<template>
  <div class="rl-empty-state" :class="`rl-empty-state--${size}`">
    <RlIcon :name="icon" :size="size === 'sm' ? 24 : 32" class="rl-empty-state__icon" aria-hidden="true" />
    <RlHeading :level="3" :size="size === 'sm' ? 'sm' : 'md'">{{ title }}</RlHeading>
    <RlText v-if="description" as="p" size="sm" tone="muted" class="rl-empty-state__description">
      {{ description }}
    </RlText>
    <div v-if="$slots.actions" class="rl-empty-state__actions"><slot name="actions" /></div>
  </div>
</template>

<style scoped>
.rl-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-8) var(--rl-space-4);
  text-align: center;
}

.rl-empty-state--sm { padding: var(--rl-space-5) var(--rl-space-4); }

.rl-empty-state__icon { color: var(--rl-color-text-subtle); }

.rl-empty-state__description {
  margin: 0;
  /* Kept short so it stays one or two lines at any width. */
  max-width: 44ch;
}

.rl-empty-state__actions { margin-top: var(--rl-space-2); }
</style>
