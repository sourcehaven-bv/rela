<script setup lang="ts">
/**
 * Overlapping row of avatars with a "+N" for the rest.
 *
 * The whole row is one labelled image rather than a series of them, so a
 * screen reader says "Assigned to A, B and 3 others" instead of reading five
 * separate avatars.
 */
import { computed } from 'vue'
import RlAvatar from './RlAvatar.vue'

const props = withDefaults(
  defineProps<{
    people: Array<{ name: string; src?: string }>
    max?: number
    size?: 'xs' | 'sm' | 'md' | 'lg'
    label?: string
  }>(),
  { max: 3, size: 'sm', label: 'Assigned to' },
)

const shown = computed(() => props.people.slice(0, props.max))
const overflow = computed(() => Math.max(0, props.people.length - props.max))

const description = computed(() => {
  const names = props.people.map((person) => person.name)
  return `${props.label}: ${names.join(', ')}`
})
</script>

<template>
  <span class="rl-avatar-group" role="img" :aria-label="description">
    <RlAvatar
      v-for="person in shown"
      :key="person.name"
      :name="person.name"
      :src="person.src"
      :size="size"
      decorative
      class="rl-avatar-group__item"
    />
    <span v-if="overflow" class="rl-avatar-group__more" :class="`rl-avatar-group__more--${size}`" aria-hidden="true">
      +{{ overflow }}
    </span>
  </span>
</template>

<style scoped>
.rl-avatar-group {
  display: inline-flex;
  align-items: center;
}

/* Each avatar overlaps the one before it and carries a ring to stay distinct. */
.rl-avatar-group__item:not(:first-child),
.rl-avatar-group__more { margin-left: -6px; }

.rl-avatar-group :deep(.rl-avatar) { box-shadow: 0 0 0 2px var(--rl-color-bg); }

.rl-avatar-group__more {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  border-radius: 50%;
  background: var(--rl-color-bg-active);
  /*
   * Full-strength text, not muted: muted grey on this surface measures 4.27,
   * which is under the 4.5 minimum for text this small.
   */
  color: var(--rl-color-text);
  font-weight: var(--rl-font-weight-medium);
  box-shadow: 0 0 0 2px var(--rl-color-bg);
}

.rl-avatar-group__more--xs { width: 20px; height: 20px; font-size: 9px; }
.rl-avatar-group__more--sm { width: 24px; height: 24px; font-size: 10px; }
.rl-avatar-group__more--md { width: 32px; height: 32px; font-size: var(--rl-font-size-xs); }
.rl-avatar-group__more--lg { width: 40px; height: 40px; font-size: var(--rl-font-size-sm); }
</style>
