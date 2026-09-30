<script setup lang="ts">
/**
 * Indeterminate bar across the top of the page while a navigation is in
 * flight. The first of the three pending indicators: this one is for moving
 * between screens, RlButton's loading state is for an explicit action, and
 * RlAutoSaveIndicator is for ambient background saving.
 *
 * Deliberately indeterminate. A fake percentage that stalls at 90 tells the
 * user less than an honest "something is happening".
 */
import { onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    active: boolean
    /**
     * Held back so a fast response never flashes a bar. Anything under this
     * feels instant, and showing a loader for it reads as slower, not faster.
     */
    delay?: number
  }>(),
  { delay: 300 },
)

const visible = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

watch(
  () => props.active,
  (active) => {
    clearTimeout(timer)
    if (active) {
      timer = setTimeout(() => (visible.value = true), props.delay)
    } else {
      visible.value = false
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <!--
    Not a live region and not a progressbar role: a screen reader user is
    told about the new page when it arrives, and announcing the wait as well
    would interrupt them twice for one navigation.
  -->
  <div v-if="visible" class="rl-activity-bar" aria-hidden="true">
    <span class="rl-activity-bar__fill" />
  </div>
</template>

<style scoped>
.rl-activity-bar {
  position: fixed;
  /*
   * Below the status bar rather than under it: a 2px line hidden by the
   * notch would report nothing at all. The width still spans the full
   * viewport, since a horizontal bar reads fine running under the corners.
   */
  top: var(--rl-safe-inset-top);
  left: 0;
  right: 0;
  z-index: var(--rl-z-toast);
  height: 2px;
  overflow: hidden;
  background: var(--rl-color-bg-active);
}

.rl-activity-bar__fill {
  display: block;
  width: 40%;
  height: 100%;
  background: var(--rl-color-accent);
  animation: rl-activity-slide 1.1s ease-in-out infinite;
}

@keyframes rl-activity-slide {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(350%); }
}

@media (prefers-reduced-motion: reduce) {
  /* Pulse in place rather than travel across the screen. */
  .rl-activity-bar__fill {
    width: 100%;
    animation: rl-activity-pulse 1.4s ease-in-out infinite;
  }
  @keyframes rl-activity-pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.35; }
  }
}
</style>
