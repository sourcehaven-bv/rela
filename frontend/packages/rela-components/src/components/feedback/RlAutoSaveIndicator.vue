<script setup lang="ts">
/**
 * Ambient save status for a form that saves as the user types.
 *
 * The third pending indicator. It is quiet by design: nothing is shown while
 * idle, because a permanent "Saved" is reassurance the user stopped reading
 * long ago. Only an error is announced, since that is the only state they
 * have to do something about.
 */
import { computed } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import RlSpinner from '../common/RlSpinner.vue'
import RlText from '../common/RlText.vue'

const props = withDefaults(
  defineProps<{
    state: 'idle' | 'saving' | 'saved' | 'error'
    /** Shown in the error state, such as `Offline. Retrying.` */
    errorText?: string
  }>(),
  { errorText: 'Could not save' },
)

const label = computed(() =>
  ({ idle: '', saving: 'Saving', saved: 'Saved', error: props.errorText })[props.state],
)
</script>

<template>
  <div
    v-if="state !== 'idle'"
    class="rl-autosave"
    :class="`rl-autosave--${state}`"
    :role="state === 'error' ? 'alert' : 'status'"
  >
    <RlSpinner v-if="state === 'saving'" :size="13" label="Saving" />
    <RlIcon v-else-if="state === 'saved'" name="done" :size="14" aria-hidden="true" />
    <RlIcon v-else name="alert" :size="14" aria-hidden="true" />
    <RlText size="sm" tone="muted">{{ label }}</RlText>
  </div>
</template>

<style scoped>
.rl-autosave {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  color: var(--rl-color-text-muted);
}

.rl-autosave--saved { color: var(--rl-color-success-fg); }
.rl-autosave--error { color: var(--rl-color-danger); }

/* The text inherits the state colour rather than staying muted. */
.rl-autosave--saved :deep(.rl-text),
.rl-autosave--error :deep(.rl-text) { color: inherit; }
</style>
