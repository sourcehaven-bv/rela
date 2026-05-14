<script setup lang="ts">
import { computed } from 'vue'
import type { SaveStatus } from '@/composables/useAutoSave'

const props = defineProps<{
  status: SaveStatus
  error?: string | null
}>()

// Always-on ambient indicator: glyph swaps with state.
// - saved/idle: check mark in circle (rela is local-first; no cloud).
// - saving: spinning ring.
// - error: warning triangle.
const tooltip = computed(() => {
  if (props.error) return props.error
  switch (props.status) {
    case 'saving':
      return 'Saving…'
    case 'saved':
      return 'Saved'
    case 'error':
      return 'Save failed'
    default:
      return 'All changes saved'
  }
})

const renderState = computed(() => {
  if (props.status === 'saving') return 'saving'
  if (props.status === 'error') return 'error'
  // idle and saved both render the same "saved" cloud — no flash on success.
  return 'saved'
})
</script>

<template>
  <div
    class="autosave-indicator"
    :class="`autosave-${renderState}`"
    :title="tooltip"
    data-testid="autosave-indicator"
    :data-status="status"
    role="status"
    aria-live="polite"
    :aria-label="tooltip"
  >
    <!-- Saved: check mark in circle (no cloud — rela is local-first) -->
    <svg
      v-if="renderState === 'saved'"
      class="autosave-icon"
      viewBox="0 0 24 24"
      width="20"
      height="20"
      aria-hidden="true"
    >
      <path
        fill="currentColor"
        d="M12 2a10 10 0 1 0 10 10A10 10 0 0 0 12 2zm-1.4 14.6L6 12l1.4-1.4 3.2 3.2 6-6L18 9.2z"
      />
    </svg>

    <!-- Saving: spinning ring -->
    <svg
      v-else-if="renderState === 'saving'"
      class="autosave-icon autosave-spin"
      viewBox="0 0 24 24"
      width="20"
      height="20"
      aria-hidden="true"
    >
      <circle
        cx="12"
        cy="12"
        r="9"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        stroke-linecap="round"
        stroke-dasharray="14 42"
      />
    </svg>

    <!-- Error: triangle with exclamation -->
    <svg
      v-else
      class="autosave-icon"
      viewBox="0 0 24 24"
      width="20"
      height="20"
      aria-hidden="true"
    >
      <path
        fill="currentColor"
        d="M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z"
      />
    </svg>
  </div>
</template>

<style scoped>
.autosave-indicator {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  user-select: none;
  cursor: default;
  transition: color 0.2s ease;
}

.autosave-icon {
  display: block;
}

.autosave-saved {
  color: var(--text-2, #888);
  opacity: 0.7;
}
.autosave-saved:hover {
  opacity: 1;
}

.autosave-saving {
  color: var(--accent, #4a90e2);
}

.autosave-error {
  color: var(--danger, #c2342f);
}

.autosave-spin {
  animation: autosave-rotate 1s linear infinite;
  transform-origin: center;
}

@keyframes autosave-rotate {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
</style>
