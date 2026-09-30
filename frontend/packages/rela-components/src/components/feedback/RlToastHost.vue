<script setup lang="ts">
/**
 * The corner where toasts stack.
 *
 * A live region wraps the list rather than each toast, so a screen reader
 * announces each new message without re-reading the ones already there.
 *
 * ## Two regions, split by tone
 *
 * `aria-live` is read from the element whose content changed, and a region's
 * politeness cannot be varied per message. So a single region would have to
 * pick one politeness for every tone.
 *
 * `polite` is right for most of them: a toast reports something that has
 * already happened and should not cut across what the user is doing. It is
 * wrong for `danger`, which reports a failure the user is likely acting on
 * the assumption of — a save that did not happen, a delete that did — and
 * which waits behind whatever is being read otherwise.
 *
 * So there are two regions, and a toast is routed by tone. Both are always in
 * the DOM: a live region has to exist before the content arrives, or the
 * insertion is not announced at all.
 *
 * Visually they are one stack. The regions are `display: contents`, so the
 * split reaches assistive technology and nothing else; the ordering between
 * the two is by tone rather than arrival, which is the one visible cost.
 */
import { computed } from 'vue'
import RlToast from './RlToast.vue'
import type { ToastMessage } from './types'

const props = withDefaults(
  defineProps<{
    toasts: ToastMessage[]
    placement?: 'top-right' | 'bottom-right' | 'top-center'
  }>(),
  { placement: 'bottom-right' },
)

defineEmits<{ dismiss: [id: string] }>()

const assertive = computed(() => props.toasts.filter((t) => t.tone === 'danger'))
const polite = computed(() => props.toasts.filter((t) => t.tone !== 'danger'))
</script>

<template>
  <Teleport to="body">
    <div
      class="rl-toast-host"
      :class="`rl-toast-host--${placement}`"
      role="region"
      aria-label="Notifications"
    >
      <!--
        Assertive first, so a failure is also the first thing reached when
        tabbing into the stack, not only the first announced.
      -->
      <div class="rl-toast-host__live" aria-live="assertive">
        <RlToast
          v-for="toast in assertive"
          :key="toast.id"
          :toast="toast"
          @dismiss="$emit('dismiss', $event)"
        />
      </div>

      <div class="rl-toast-host__live" aria-live="polite">
        <RlToast
          v-for="toast in polite"
          :key="toast.id"
          :toast="toast"
          @dismiss="$emit('dismiss', $event)"
        />
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.rl-toast-host {
  position: fixed;
  z-index: var(--rl-z-toast);
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
  width: min(380px, calc(100vw - var(--rl-space-8)));
  /*
   * The region is always in the DOM so the live area exists before the first
   * message arrives, but it must not block clicks while it is empty.
   */
  pointer-events: none;
}

/*
 * The live regions are grouping for assistive technology only; the flex
 * column above has to keep laying out the toasts themselves, so the wrappers
 * take themselves out of the box tree.
 */
.rl-toast-host__live { display: contents; }

.rl-toast-host .rl-toast { pointer-events: auto; }

/*
 * Each corner is offset from the edge it hangs off, plus whatever that edge
 * is obscured by: the home indicator at the bottom, the status bar at the
 * top, the notch at the sides in landscape.
 */
.rl-toast-host--bottom-right {
  right: calc(var(--rl-space-4) + var(--rl-safe-inset-right));
  bottom: calc(var(--rl-space-4) + var(--rl-safe-inset-bottom));
}
.rl-toast-host--top-right {
  right: calc(var(--rl-space-4) + var(--rl-safe-inset-right));
  top: calc(var(--rl-space-4) + var(--rl-safe-inset-top));
}
.rl-toast-host--top-center {
  top: calc(var(--rl-space-4) + var(--rl-safe-inset-top));
  left: 50%;
  transform: translateX(-50%);
}

@media (max-width: 767px) {
  /* Full width at the bottom, clear of the thumb reach at the very edge. */
  .rl-toast-host {
    left: calc(var(--rl-space-4) + var(--rl-safe-inset-left));
    right: calc(var(--rl-space-4) + var(--rl-safe-inset-right));
    width: auto;
    transform: none;
  }
}
</style>
