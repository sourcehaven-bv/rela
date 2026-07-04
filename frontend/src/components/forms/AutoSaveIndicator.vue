<script setup lang="ts">
import { computed } from 'vue'
import type { SaveStatus } from '@/composables/useAutoSave'

const props = defineProps<{
  status: SaveStatus
  error?: string | null
}>()

// Hidden-until-needed ambient indicator: glyph swaps with state, and the
// wrapper is only opaque while something is happening.
// - idle: invisible (opacity 0) — nothing to report; the check for the
//   preceding save has already faded out via SAVED_INDICATOR_MS upstream.
// - saved: check mark in circle (rela is local-first; no cloud). Shown
//   briefly after a save, then upstream flips status back to idle, which
//   fades this out (transition on .autosave-indicator).
// - saving: spinning ring.
// - error: warning triangle (persists until resolved).
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
  if (props.error || props.status === 'error') return 'error'
  if (props.status === 'saving') return 'saving'
  if (props.status === 'saved') return 'saved'
  // idle: render the "saved" glyph but hidden, so the saved → idle flip is a
  // fade-out rather than a glyph swap.
  return 'saved'
})

// Visible while saving/saved/error; invisible (faded out) when idle. Kept in
// the DOM either way so the opacity transition can run.
const visible = computed(() => props.error != null || props.status !== 'idle')

// Screen-reader announcement. A live region announces its TEXT CONTENT, not
// attribute changes — the glyphs are aria-hidden SVGs, so state is conveyed
// through this visually-hidden text node instead (RR-4SN00Y). Empty at idle
// so the live region has nothing to announce on mount or when settling back
// to idle; only saving/saved/error produce a message.
const announcement = computed(() => {
  if (props.error != null || props.status === 'error') return 'Save failed'
  switch (props.status) {
    case 'saving':
      return 'Saving…'
    case 'saved':
      return 'Saved'
    default:
      return ''
  }
})
</script>

<template>
  <div
    class="autosave-indicator"
    :class="[`autosave-${renderState}`, { 'autosave-hidden': !visible }]"
    :title="tooltip"
    data-testid="autosave-indicator"
    :data-status="status"
    aria-hidden="true"
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

  <!--
    Visually-hidden live region: announces the save state via changing TEXT
    (the glyph SVGs are aria-hidden). Empty at idle so nothing is announced
    on mount or when settling back to idle (RR-4SN00Y).
  -->
  <span class="autosave-sr-only" role="status" aria-live="polite">{{ announcement }}</span>
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
  transition:
    color 0.2s ease,
    opacity 0.3s ease;
}

/* Hidden-until-needed: idle fades out over 0.3s. Stays in the DOM (opacity,
   not v-if) so the saved → idle transition animates. pointer-events off so
   the invisible glyph isn't hoverable. */
.autosave-hidden {
  opacity: 0;
  pointer-events: none;
}

/* Visually hidden but exposed to assistive tech (the standard sr-only clip). */
.autosave-sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

@media (prefers-reduced-motion: reduce) {
  .autosave-indicator {
    transition: none;
  }
}

.autosave-icon {
  display: block;
}

.autosave-saved {
  color: var(--text-2, #888);
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
