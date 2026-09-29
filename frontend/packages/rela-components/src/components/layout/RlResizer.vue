<script setup lang="ts">
/**
 * The draggable edge between two panes.
 *
 * It reports a width and stores nothing. Where that width is kept, and
 * whether it survives a reload, is the application's: a component that wrote
 * to `localStorage` would share one key between every instance on the page,
 * which is the coupling `check:purity` exists to catch.
 *
 * It is also not inside the pane it sizes. `RlSidebar` takes its width from
 * `--rl-sidebar-width` and has no width of its own, so the resizer sits
 * beside it and the app rebinds the token. That keeps one rule about how wide
 * the sidebar is, rather than a prop and a token that can disagree.
 *
 * Clamping belongs to the caller too, and should be done in CSS rather than
 * here:
 *
 *   width: clamp(200px, var(--rl-sidebar-width), 40vw)
 *
 * A stored width that is re-clamped at render survives a small window instead
 * of being overwritten by it. Storing a width per breakpoint looks like the
 * answer and is not: the buckets go stale against each other, and a window
 * dragged across a boundary makes the pane jump.
 *
 * The keyboard is not an afterthought here. A drag handle is the easiest
 * control in a layout to leave unreachable, so this is a focusable
 * `separator` with `aria-valuenow`: arrows nudge, Home and End take the ends,
 * and Enter resets to the default.
 */
import { computed, ref } from 'vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = withDefaults(
  defineProps<{
    /** Current width of the pane, in pixels. */
    modelValue: number
    /** Narrowest the pane may be dragged. */
    min?: number
    /** Widest the pane may be dragged. */
    max?: number
    /**
     * Which side of the handle the pane being sized is on. `start` for a
     * left sidebar, where dragging right widens it; `end` for a right-hand
     * detail panel, where dragging right narrows it.
     */
    side?: 'start' | 'end'
    /** Names the handle for assistive tech. */
    label?: string
    /**
     * Width to return to on Enter or a double-click. Leave unset for no
     * reset, which is right where there is no meaningful default.
     */
    defaultValue?: number
    /** Pixels moved per arrow press. */
    step?: number
    /** Pixels moved per Page Up or Page Down. */
    pageStep?: number
    disabled?: boolean
  }>(),
  {
    min: 180,
    max: 480,
    side: 'start',
    label: 'Resize panel',
    step: 16,
    pageStep: 64,
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: number]
  /** A drag began. For a caller that wants to suppress transitions during it. */
  'resize-start': []
  /**
   * A drag ended. The width has already been emitted; this marks the end of a
   * run of them, for a caller that would rather save once than once a frame.
   */
  'resize-end': []
}>()

const dragging = ref(false)

const clamp = (value: number) => Math.min(props.max, Math.max(props.min, Math.round(value)))

function set(value: number) {
  const next = clamp(value)
  if (next !== props.modelValue) emit('update:modelValue', next)
}

/*
 * Pointer events with capture rather than mouse events on `window`: capture
 * keeps the stream coming to this element even when the pointer outruns it,
 * which it will, because a drag moves faster than a layout settles. It also
 * covers touch and pen without a second code path.
 */
function onPointerdown(event: PointerEvent) {
  if (props.disabled || event.button !== 0) return
  event.preventDefault()
  const handle = event.currentTarget as HTMLElement
  handle.setPointerCapture(event.pointerId)
  dragging.value = true
  emit('resize-start')
}

function onPointermove(event: PointerEvent) {
  if (!dragging.value) return
  const handle = event.currentTarget as HTMLElement
  const rect = handle.getBoundingClientRect()
  /*
   * Measured from the handle's own centre each frame rather than from a
   * remembered offset at the press. The handle moves as the pane resizes, so
   * the two agree by construction and the width cannot drift from the cursor
   * over a long drag.
   */
  const delta = event.clientX - (rect.left + rect.width / 2)
  set(props.modelValue + (props.side === 'start' ? delta : -delta))
}

function onPointerup(event: PointerEvent) {
  if (!dragging.value) return
  const handle = event.currentTarget as HTMLElement
  if (handle.hasPointerCapture(event.pointerId)) handle.releasePointerCapture(event.pointerId)
  dragging.value = false
  emit('resize-end')
}

/* Arrows read as "widen" and "narrow", so they follow the side the pane is on. */
function nudge(towardsEnd: number, amount: number) {
  set(props.modelValue + (props.side === 'start' ? towardsEnd : -towardsEnd) * amount)
}

function onKeydown(event: KeyboardEvent) {
  if (props.disabled) return
  switch (event.key) {
    case 'ArrowLeft':
      event.preventDefault()
      nudge(-1, props.step)
      break
    case 'ArrowRight':
      event.preventDefault()
      nudge(1, props.step)
      break
    case 'PageUp':
      event.preventDefault()
      nudge(1, props.pageStep)
      break
    case 'PageDown':
      event.preventDefault()
      nudge(-1, props.pageStep)
      break
    case 'Home':
      event.preventDefault()
      set(props.min)
      break
    case 'End':
      event.preventDefault()
      set(props.max)
      break
    case 'Enter':
      if (props.defaultValue === undefined) return
      event.preventDefault()
      set(props.defaultValue)
      break
  }
}

function reset() {
  if (props.disabled || props.defaultValue === undefined) return
  set(props.defaultValue)
}

/*
 * A separator that can be moved is announced as a slider would be, which is
 * what a screen reader needs to say anything useful about it. The value is
 * the width in pixels; `aria-valuetext` says so, because "260" on its own
 * could be anything.
 */
const valueText = computed(() => messages.resizerValue({ pixels: props.modelValue }))
</script>

<template>
  <div
    class="rl-resizer"
    :class="{ 'rl-resizer--dragging': dragging, 'rl-resizer--disabled': disabled }"
    role="separator"
    :tabindex="disabled ? undefined : 0"
    :aria-label="label"
    aria-orientation="vertical"
    :aria-valuenow="modelValue"
    :aria-valuemin="min"
    :aria-valuemax="max"
    :aria-valuetext="valueText"
    :aria-disabled="disabled || undefined"
    @pointerdown="onPointerdown"
    @pointermove="onPointermove"
    @pointerup="onPointerup"
    @pointercancel="onPointerup"
    @dblclick="reset"
    @keydown="onKeydown"
  >
    <!--
      The line is drawn inside a wider, transparent handle: a 1px target is
      unhittable, and a visible 8px divider would be a heavy border between
      every pair of panes for the sake of a rare drag.
    -->
    <span class="rl-resizer__line" aria-hidden="true" />

    <!--
      The grip is what says the edge can be moved. Without it the handle is a
      1px line the same colour as an ordinary border, so the only thing
      advertising the drag is a cursor that appears once the pointer is
      already on it, which nobody discovers by accident.
    -->
    <span class="rl-resizer__grip" aria-hidden="true" />
  </div>
</template>

<style scoped>
.rl-resizer {
  position: relative;
  flex: none;
  width: 9px;
  /* Pulled into its neighbours so the grab area costs no layout width. */
  margin-inline: -4px;
  z-index: 1;
  cursor: col-resize;
  touch-action: none;
}

.rl-resizer--disabled { cursor: default; }

.rl-resizer__line {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 4px;
  width: 1px;
  background: var(--rl-color-border);
}

/*
 * A short bar at the vertical centre, drawn as a pill rather than as dots:
 * at 3px wide a dotted grip is mostly gaps and reads as a rendering fault.
 * Centred, because that is where a hand goes for an edge whose whole height
 * is the target.
 */
.rl-resizer__grip {
  position: absolute;
  top: 50%;
  left: 3px;
  width: 3px;
  height: 28px;
  transform: translateY(-50%);
  border-radius: var(--rl-radius-pill);
  background: var(--rl-color-border-strong);
  transition: background var(--rl-duration-fast) var(--rl-ease), height var(--rl-duration-fast) var(--rl-ease);
}

/*
 * The grip carries the state on its own and the line stays 1px. Thickening
 * the full height as well drew an accent bar down the whole viewport, which
 * swamped the grip it was meant to reinforce and read as a selected pane
 * rather than as an edge under the pointer.
 */
.rl-resizer:hover .rl-resizer__grip,
.rl-resizer--dragging .rl-resizer__grip {
  height: 44px;
  background: var(--rl-color-accent);
}

.rl-resizer:focus-visible {
  outline: none;
}

.rl-resizer:focus-visible .rl-resizer__grip {
  height: 44px;
  background: var(--rl-color-focus);
}

/* Nothing to grip when it cannot be moved. */
.rl-resizer--disabled .rl-resizer__grip { display: none; }

@media (prefers-reduced-motion: reduce) {
  .rl-resizer__grip { transition: none; }
}

/*
 * A fine drag is a pointing-device act. On touch the handle is left in place
 * for its keyboard and assistive behaviour but widened to something a finger
 * can find, since a 9px target is below every tap-size guideline.
 */
@media (pointer: coarse) {
  .rl-resizer {
    width: var(--rl-tap-target);
    margin-inline: calc(var(--rl-tap-target) / -2 + 0.5px);
  }

  .rl-resizer__line { left: calc(var(--rl-tap-target) / 2 - 0.5px); }

  .rl-resizer__grip { left: calc(var(--rl-tap-target) / 2 - 1.5px); }
}
</style>
