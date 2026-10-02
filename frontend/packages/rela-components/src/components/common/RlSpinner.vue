<script setup lang="ts">
/** Indeterminate activity indicator, sized to sit inline with button text. */
withDefaults(defineProps<{ size?: number; label?: string }>(), {
  size: 15,
  label: 'Loading',
})
</script>

<template>
  <span class="rl-spinner" role="status" :aria-label="label">
    <svg
      class="rl-spinner__svg"
      :width="size"
      :height="size"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-opacity="0.25" stroke-width="2.5" />
      <path
        d="M21 12a9 9 0 0 0-9-9"
        stroke="currentColor"
        stroke-width="2.5"
        stroke-linecap="round"
      />
    </svg>
  </span>
</template>

<style scoped>
.rl-spinner {
  display: inline-flex;
  flex: none;
  color: inherit;
}

.rl-spinner__svg {
  animation: rl-spin 750ms linear infinite;
}

@keyframes rl-spin {
  to { transform: rotate(360deg); }
}

/* Respect a reduced-motion preference: fade rather than spin. */
@media (prefers-reduced-motion: reduce) {
  .rl-spinner__svg {
    animation: rl-spinner-pulse 1.4s ease-in-out infinite;
  }

  @keyframes rl-spinner-pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }
}
</style>
