<script setup lang="ts">
import { useDelayedPending } from '@/composables/useDelayedPending'
import { PENDING_TIMINGS } from '@/composables/pendingTimings'

// Global navigation activity indicator (TKT-TFSNBY).
//
// The navigation class of the three-indicator model: a route change is the
// user's own act, but the indicator is peripheral rather than at the
// cursor, so it is the LEAST invasive of the three and gets the shortest
// delay. Explicit actions (button label swaps) and ambient autosave own
// their own surfaces and never drive this bar — exactly one indicator per
// user act.
//
// Layout stability comes free from `position: fixed`: per the CLS
// definition a change only counts as a shift if it moves other elements'
// start positions, and a fixed-position overlay moves nothing. That is the
// main structural argument for a top bar over any inline spinner.
//
// The bar is indeterminate on purpose. rela's slow case is 1-2s, which is
// far below the band where a determinate percentage earns its keep, and we
// have no progress signal from the router anyway.

const props = defineProps<{
  /** True while a navigation is in flight. */
  active: boolean
}>()

const visible = useDelayedPending(
  () => props.active,
  {
    delay: PENDING_TIMINGS.navDelayMs,
    minDuration: PENDING_TIMINGS.navMinDurationMs,
  }
)
</script>

<template>
  <!--
    aria-hidden: the route change announces itself through the destination
    view's own content and heading. A live region here would double up, and
    "loading" announcements on every navigation are noise rather than help.
  -->
  <div
    class="activity-bar"
    :class="{ 'activity-bar--visible': visible }"
    data-testid="activity-bar"
    aria-hidden="true"
  >
    <div class="activity-bar__fill" />
  </div>
</template>

<style scoped>
.activity-bar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  /* Above the sidebar (101) and mobile bars (102), below modals (1000):
     navigation is app-level, but a modal is closer to the user. */
  z-index: 200;
  /* Never intercepts clicks — it is a status surface, not a control. */
  pointer-events: none;
  opacity: 0;
  /* Opacity, not display/v-if: the element keeps its (zero-height, fixed)
     box either way, and a fade means a bar that appears right at the
     threshold eases in rather than snapping. */
  transition: opacity var(--pending-fade) ease-in;
}

.activity-bar--visible {
  opacity: 1;
}

.activity-bar__fill {
  height: 100%;
  width: 100%;
  background: var(--accent-color, #4772fb);
  transform-origin: 0 50%;
  animation: activity-bar-slide 1.4s ease-in-out infinite;
}

/* Indeterminate sweep: no progress signal exists, so the motion conveys
   liveness only. */
@keyframes activity-bar-slide {
  0% {
    transform: scaleX(0);
  }
  50% {
    transform: scaleX(0.7);
  }
  100% {
    transform: scaleX(1);
  }
}

@media (prefers-reduced-motion: reduce) {
  .activity-bar {
    transition: none;
  }

  /* Hold a static bar rather than sweeping. The STATE is still conveyed —
     only the motion is removed, which is the point of the preference. */
  .activity-bar__fill {
    animation: none;
    transform: scaleX(1);
  }
}
</style>
