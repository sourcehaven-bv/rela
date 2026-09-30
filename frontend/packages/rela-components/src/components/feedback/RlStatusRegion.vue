<script setup lang="ts">
/**
 * What a pane shows instead of its content, while it loads or after it failed.
 *
 * This replaces a region rather than sitting inside one, which is what
 * separates it from the other feedback components. A banner or a callout is a
 * message alongside content that is present; this is what stands where the
 * content would be. An empty state is the third case: the pane loaded, it
 * worked, and the answer is that there is nothing to show.
 *
 * # Why the tone decides the announcement
 *
 * A screen reader user who cannot see the pane go blank has only the live
 * region to tell them what happened. `pending` is polite, because a load that
 * is about to finish should not interrupt what is being read; `error` is
 * assertive, because it ends the wait and the user is otherwise left
 * expecting content that is never coming. `info` announces nothing: it is a
 * standing statement about the pane, not an event.
 *
 * # Why `pending` brings its own spinner
 *
 * A wait with no visible motion reads as a finished screen that happens to be
 * empty. Every caller wants the spinner, so the tone supplies it instead of
 * each caller remembering to pass one.
 */
import { computed } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import RlSpinner from '../common/RlSpinner.vue'
import RlText from '../common/RlText.vue'
import type { IconName } from '../common/icons'

const props = withDefaults(
  defineProps<{
    /**
     * `pending` is a pane that has not loaded, `error` one that failed, and
     * `info` a pane that is unavailable for a reason that is neither: no
     * workspace selected, a feature switched off.
     */
    tone?: 'pending' | 'error' | 'info'
    /**
     * Replaces the tone's own mark. `pending` shows a spinner and the other
     * tones an icon, so naming an icon here also turns the spinner off.
     */
    icon?: IconName
    /** `sm` for a side panel, `md` for a whole view. */
    size?: 'sm' | 'md'
    /**
     * What the spinner announces while the tone is `pending`. The message in
     * the slot is prose rather than a name, so a wait for something other than
     * a load says so here: "Analysing", "Saving".
     */
    pendingLabel?: string
  }>(),
  { tone: 'pending', size: 'md', pendingLabel: 'Loading' },
)

const toneIcons = { error: 'alert', info: 'info' } as const

/*
 * A spinner for the wait, an icon for a state that has settled. Naming an
 * icon overrides both, for a wait that is better described by what it is
 * waiting on than by motion.
 *
 * Computed rather than read once, because the tone changing in place is the
 * normal case: the same region is the wait and then the failure.
 */
const spinning = computed(() => props.tone === 'pending' && !props.icon)

const mark = computed<IconName | null>(
  () => props.icon ?? (props.tone === 'pending' ? null : toneIcons[props.tone]),
)

/*
 * `alert` is assertive and `status` polite; `info` gets neither, because a
 * standing description of the pane is not something that just happened. While
 * the spinner is showing it carries its own `role="status"`, so naming the
 * region as well would announce the same wait twice.
 */
const role = computed(() => {
  if (props.tone === 'error') return 'alert'
  if (props.tone === 'pending' && !spinning.value) return 'status'
  return undefined
})
</script>

<template>
  <div
    class="rl-status-region"
    :class="[`rl-status-region--${tone}`, `rl-status-region--${size}`]"
    :role="role"
  >
    <RlSpinner
      v-if="spinning"
      :size="size === 'sm' ? 20 : 24"
      :label="pendingLabel"
      class="rl-status-region__spinner"
    />
    <RlIcon
      v-else-if="mark"
      :name="mark"
      :size="size === 'sm' ? 20 : 24"
      class="rl-status-region__icon"
      aria-hidden="true"
    />

    <RlText as="p" size="sm" tone="muted" class="rl-status-region__message"><slot /></RlText>

    <div v-if="$slots.actions" class="rl-status-region__actions"><slot name="actions" /></div>
  </div>
</template>

<style scoped>
.rl-status-region {
  display: flex;
  flex: 1;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--rl-space-4);
  padding: var(--rl-space-8) var(--rl-space-4);
  text-align: center;
}

/* A side panel has no room for the full padding, and no need for it. */
.rl-status-region--sm {
  gap: var(--rl-space-3);
  padding: var(--rl-space-6) var(--rl-space-4);
}

/*
 * The spinner stays neutral: a wait is not a severity. Only the error tone
 * colours its mark, and the message stays muted in both, so a failed pane
 * does not read as more alarming than the thing that failed.
 */
.rl-status-region__spinner { color: var(--rl-color-text-subtle); }

.rl-status-region__icon { color: var(--rl-color-text-subtle); }

.rl-status-region--error .rl-status-region__icon { color: var(--rl-color-danger); }

.rl-status-region__message {
  margin: 0;
  /* Kept short so the line stays readable at a pane's full width. */
  max-width: 44ch;
}
</style>
