<script setup lang="ts">
/**
 * Short hint shown on hover and on focus.
 *
 * A tooltip is a supplement, never the only place a meaning lives: a control
 * it describes still needs its own accessible name. It appears on focus as
 * well as hover so a keyboard user gets it too (WCAG 1.4.13), and it is wired
 * with `aria-describedby` so it is announced rather than merely seen.
 */
import { computed, ref, useId } from 'vue'
import { useAnchoredPanel } from '../../composables/useAnchoredPanel'

const props = withDefaults(
  defineProps<{
    text: string
    /** Preferred side. The bubble flips when that side has no room. */
    placement?: 'top' | 'bottom' | 'left' | 'right'
    /** Milliseconds before it appears, so a pointer crossing the page is quiet. */
    delay?: number
  }>(),
  { placement: 'top', delay: 200 },
)

const id = useId()
const visible = ref(false)
const root = ref<HTMLElement | null>(null)
const bubble = ref<HTMLElement | null>(null)
let timer: ReturnType<typeof setTimeout> | undefined

/*
 * No height clamp: a hint that scrolls is worse than one on the other side,
 * and the bubble is small enough that flipping always finds room.
 */
const { floatingStyles } = useAnchoredPanel(root, bubble, {
  placement: computed(() => props.placement),
  gap: 8,
  clampHeight: false,
})

function show(immediate = false) {
  clearTimeout(timer)
  /*
   * Focus shows it at once: a keyboard user has already committed to the
   * control, so the delay that filters out a passing pointer only gets in
   * their way.
   */
  timer = setTimeout(() => (visible.value = true), immediate ? 0 : props.delay)
}

function hide() {
  clearTimeout(timer)
  visible.value = false
}
</script>

<template>
  <span
    ref="root"
    class="rl-tooltip"
    @mouseenter="show()"
    @mouseleave="hide"
    @focusin="show(true)"
    @focusout="hide"
    @keydown.escape="hide"
  >
    <!-- The caller must spread `describedBy` onto the control it wraps. -->
    <slot :described-by="id" />

    <!-- Teleported so a clipping ancestor cannot cut the bubble off. -->
    <Teleport to="body">
      <span
        v-if="visible"
        :id="id"
        ref="bubble"
        role="tooltip"
        class="rl-panel rl-panel--tooltip"
        :style="floatingStyles"
      >
        {{ text }}
      </span>
    </Teleport>
  </span>
</template>

<style scoped>
.rl-tooltip {
  position: relative;
  display: inline-flex;
}

/* The bubble is styled in styles/panel.css: it is teleported, so a scoped
   rule here would not reach it. */
</style>
