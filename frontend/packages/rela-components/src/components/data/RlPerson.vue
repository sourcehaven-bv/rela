<script setup lang="ts">
/**
 * A person, drawn as their avatar and their name.
 *
 * The pair had been hand-rolled everywhere it appeared: a task card, a
 * subtask row, a detail field and a table cell each built their own, which is
 * why one of them showed the name with no avatar and another showed an avatar
 * with no accessible name. This is the one shape, so a person looks the same
 * wherever they are mentioned.
 *
 * `unassigned` is a state, not an absence. A row with nothing in it reads as
 * a rendering fault; a dimmed "Unassigned" says the field was looked at and
 * is empty on purpose.
 *
 * The avatar is always decorative here, because the name is written beside
 * it. When the name is hidden it moves onto the wrapper instead, so the
 * person is announced exactly once either way.
 */
import { computed } from 'vue'
import RlAvatar from './RlAvatar.vue'
import type { Person } from './types'

const props = withDefaults(
  defineProps<{
    /** The person, or nothing for the unassigned state. */
    person?: Person
    size?: 'xs' | 'sm' | 'md' | 'lg'
    /**
     * Drops the name, leaving the avatar. For a dense row or a stack of
     * avatars where the name is shown elsewhere or on hover.
     */
    nameHidden?: boolean
    /** Second line under the name, such as a role or an email address. */
    secondary?: string
    /**
     * Drops the second line, keeping the name. For a one-line box such as a
     * picker's closed value, where the person's own `secondary` would make
     * the row two lines tall and overflow whatever is holding it.
     */
    secondaryHidden?: boolean
    /** Wording for the empty state. */
    emptyLabel?: string
  }>(),
  { size: 'sm', nameHidden: false, secondaryHidden: false, emptyLabel: 'Unassigned' },
)

const label = computed(() => props.person?.name ?? props.emptyLabel)
</script>

<template>
  <span
    class="rl-person"
    :class="[`rl-person--${size}`, { 'rl-person--empty': !person }]"
    :role="nameHidden ? 'img' : undefined"
    :aria-label="nameHidden ? label : undefined"
  >
    <RlAvatar :name="person?.name" :src="person?.avatarUrl" :size="size" decorative />

    <span v-if="!nameHidden" class="rl-person__text">
      <span class="rl-person__name">{{ label }}</span>
      <span
        v-if="!secondaryHidden && (secondary ?? person?.secondary)"
        class="rl-person__secondary"
      >
        {{ secondary ?? person?.secondary }}
      </span>
    </span>
  </span>
</template>

<style scoped>
.rl-person {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
  min-width: 0;
}

.rl-person__text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

/* One line each: a name is allowed to be long, a row is not allowed to grow. */
.rl-person__name,
.rl-person__secondary {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-person__secondary {
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-subtle);
}

/* Muted rather than absent, so an empty field still reads as a field. */
.rl-person--empty .rl-person__name { color: var(--rl-color-text-subtle); }

.rl-person--xs { font-size: var(--rl-font-size-xs); }
.rl-person--sm { font-size: var(--rl-font-size-sm); }
.rl-person--md { font-size: var(--rl-font-size-md); }
.rl-person--lg { font-size: var(--rl-font-size-md); }
</style>
