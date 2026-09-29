<script setup lang="ts">
/**
 * Person marker: a photo if there is one, otherwise their initials.
 *
 * The colour is derived from the name rather than stored, so the same person
 * is the same colour everywhere without anyone having to assign one. It is
 * decorative: the name is what carries the meaning.
 */
import { computed } from 'vue'
import RlIcon from '../common/RlIcon.vue'

const props = withDefaults(
  defineProps<{
    name?: string
    src?: string
    size?: 'xs' | 'sm' | 'md' | 'lg'
    /**
     * Hides the avatar from screen readers. Correct when the name is already
     * written beside it, which is the common case in a list.
     */
    decorative?: boolean
  }>(),
  { size: 'md', decorative: false },
)

/** At most two letters: more does not fit and stops reading as initials. */
const initials = computed(() =>
  (props.name ?? '')
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? '')
    .join(''),
)

const palette = ['grey', 'blue', 'green', 'amber', 'red', 'purple'] as const

const swatch = computed(() => {
  if (!props.name) return 'grey'
  // Sum of code points: stable across sessions and cheap, which is all it needs.
  const hash = [...props.name].reduce((total, char) => total + char.charCodeAt(0), 0)
  return palette[hash % palette.length]
})
</script>

<template>
  <span
    class="rl-avatar"
    :class="[`rl-avatar--${size}`, !src && `rl-avatar--${swatch}`]"
    :role="decorative || !name ? undefined : 'img'"
    :aria-label="decorative || !name ? undefined : name"
    :aria-hidden="decorative || !name || undefined"
  >
    <img v-if="src" :src="src" alt="" class="rl-avatar__image" />
    <template v-else-if="initials">{{ initials }}</template>
    <RlIcon v-else name="user" :size="14" aria-hidden="true" />
  </span>
</template>

<style scoped>
.rl-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  overflow: hidden;
  border-radius: 50%;
  font-weight: var(--rl-font-weight-medium);
  /* Initials are uppercase already; this keeps a stray lowercase in line. */
  text-transform: uppercase;
  user-select: none;
}

.rl-avatar__image { width: 100%; height: 100%; object-fit: cover; }

.rl-avatar--xs { width: 20px; height: 20px; font-size: 9px; }
.rl-avatar--sm { width: 24px; height: 24px; font-size: 10px; }
.rl-avatar--md { width: 32px; height: 32px; font-size: var(--rl-font-size-xs); }
.rl-avatar--lg { width: 40px; height: 40px; font-size: var(--rl-font-size-sm); }

/* Reuses the tag palette, whose foregrounds already clear 4.5:1 on their own background. */
.rl-avatar--grey { background: var(--rl-tag-grey-bg); color: var(--rl-tag-grey-fg); }
.rl-avatar--blue { background: var(--rl-tag-blue-bg); color: var(--rl-tag-blue-fg); }
.rl-avatar--green { background: var(--rl-tag-green-bg); color: var(--rl-tag-green-fg); }
.rl-avatar--amber { background: var(--rl-tag-amber-bg); color: var(--rl-tag-amber-fg); }
.rl-avatar--red { background: var(--rl-tag-red-bg); color: var(--rl-tag-red-fg); }
.rl-avatar--purple { background: var(--rl-tag-purple-bg); color: var(--rl-tag-purple-fg); }
</style>
