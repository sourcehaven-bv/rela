<script setup lang="ts">
/**
 * A stack of panels that slide out from the left edge over the page.
 *
 * A click in the sidebar opens the first panel; a click on a row inside that
 * panel opens the second beside it. Closing one reveals whatever was under
 * it, which is the page itself once the last one goes.
 *
 * The stack holds no state. The caller passes the open panels as an array
 * and the stack draws them in order, so opening the second panel is pushing
 * an entry and closing it is popping one. That is deliberate: which panel is
 * open is usually part of the route, and a component that remembered it
 * would fight the URL over who decides.
 *
 * `close` names the panel to remove rather than emitting a bare event,
 * because a close always has a subject: the header's own control, an Escape,
 * or a click on the page beside it all name a level of the stack.
 *
 * The stack fills its containing block, so put it in a positioned wrapper
 * around the content the panels should cover. A wrapper that starts after
 * the sidebar gives panels that slide out of the sidebar; one that includes
 * the sidebar gives panels that slide over it. Neither is wrong, and the
 * component cannot tell which was meant, so the wrapper decides.
 */
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import RlSlidePanel from './RlSlidePanel.vue'

export interface SlidePanel {
  id: string
  title: string
  size?: 'sm' | 'md' | 'lg'
  titleHidden?: boolean
  /** Whether this panel offers a close control. */
  closable?: boolean
}

const props = withDefaults(
  defineProps<{
    /** The open panels, outermost first. An empty array shows the page alone. */
    panels: SlidePanel[]
    /**
     * Whether Escape closes the innermost panel. On by default, because a
     * panel the user can open from the keyboard has to be closable from it.
     */
    closeOnEscape?: boolean
    /**
     * Whether a click on the page beside the stack closes it. Off by default:
     * these panels sit over content the user is meant to keep using, and a
     * stack that vanished on the first click into the page would undo the
     * thing that makes it different from a modal.
     */
    closeOnClickOutside?: boolean
    /**
     * Whether to move focus into each newly opened panel, and back to
     * whatever had focus before the stack opened once the last one closes.
     *
     * Off by default, because these panels are read alongside the page: a
     * stack that grabbed the keyboard would take it away from work the user
     * is still doing, which is the thing that separates this from a drawer.
     * Turn it on where the panel is the task rather than a companion to it.
     *
     * Not a focus trap either way. Tab leaves the panel and continues into
     * the page, because the page is still live.
     */
    manageFocus?: boolean
  }>(),
  { closeOnEscape: true, closeOnClickOutside: false, manageFocus: false },
)

const emit = defineEmits<{
  /** A panel was dismissed. Drop it and everything it opened. */
  close: [id: string]
}>()

/*
 * Each panel starts where the one before it ends, so the stack reads as a
 * row of columns rather than a pile. The offsets are computed from the
 * width tokens rather than measured, for the same reason the swimlane board
 * aligns from tokens: a measured layout settles a frame late, and the panel
 * would visibly jump into place as it slid.
 */
const WIDTH_TOKEN = {
  sm: 'var(--rl-slide-panel-width-sm)',
  md: 'var(--rl-slide-panel-width-md)',
  lg: 'var(--rl-slide-panel-width-lg)',
} as const

const offsets = computed(() => {
  const result: string[] = []
  const widths: string[] = []
  for (const panel of props.panels) {
    result.push(widths.length ? `calc(${widths.join(' + ')})` : '0px')
    widths.push(WIDTH_TOKEN[panel.size ?? 'md'])
  }
  return result
})

function closeInnermost() {
  const innermost = props.panels[props.panels.length - 1]
  if (innermost) emit('close', innermost.id)
}

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  /*
   * One Escape closes one panel, so a three-panel stack takes three presses
   * to get back to the page. Collapsing the whole stack on one press would
   * throw away a navigation the user spent two clicks building.
   */
  event.stopPropagation()
  closeInnermost()
}

/*
 * Listened for on the document rather than on the stack, because focus is
 * usually in the page beside the panels: this is not a focus trap, and a
 * handler on the stack's own element would only fire when the user had
 * already clicked into it.
 */
watch(
  () => props.closeOnEscape && props.panels.length > 0,
  (active) => {
    if (typeof document === 'undefined') return
    if (active) document.addEventListener('keydown', onKeydown)
    else document.removeEventListener('keydown', onKeydown)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (typeof document === 'undefined') return
  document.removeEventListener('keydown', onKeydown)
})

function onScrimClick() {
  if (props.closeOnClickOutside) closeInnermost()
}

/*
 * Where focus was before the stack opened, so the last close can put it
 * back. Captured on the way from zero panels to some, which is the only
 * moment the page still holds focus; reading it later would return focus to
 * the panel that was closing rather than to the control that started this.
 */
const root = ref<HTMLElement | null>(null)
const returnTo = ref<HTMLElement | null>(null)

function focusPanel(index: number) {
  const panel = root.value?.querySelectorAll<HTMLElement>('.rl-slide-panel')[index]
  if (!panel) return
  /*
   * The panel itself, not the first control in it. Focusing a control would
   * skip the heading that says what just opened, and the panel carries
   * `tabindex="-1"` so it can take focus without joining the tab order.
   */
  panel.focus()
}

watch(
  () => props.panels.length,
  async (count, before) => {
    if (!props.manageFocus || typeof document === 'undefined') return

    if (count > (before ?? 0)) {
      if (!before) {
        const active = document.activeElement
        returnTo.value = active instanceof HTMLElement ? active : null
      }
      /* Two ticks: one for the panel to mount, one for its transition to start. */
      await nextTick()
      focusPanel(count - 1)
      return
    }

    if (count === 0) {
      const target = returnTo.value
      returnTo.value = null
      /*
       * Only if the control is still there and still focusable. A panel
       * opened from a row that the panel itself removed leaves nothing to go
       * back to, and forcing focus onto a detached node drops it to the body
       * silently.
       */
      if (target?.isConnected) target.focus()
      return
    }

    /* Closed one of several: focus the panel now innermost. */
    await nextTick()
    focusPanel(count - 1)
  },
)

defineSlots<{
  /** The contents of one panel, called once per open panel. */
  panel?: (props: { panel: SlidePanel; index: number }) => unknown
  /**
   * Controls for one panel's header, left of its close button. Called once
   * per open panel, so the same slot can give each panel different controls
   * by reading `panel`.
   */
  panelActions?: (props: { panel: SlidePanel; index: number }) => unknown
  'panel-actions'?: (props: { panel: SlidePanel; index: number }) => unknown
}>()
</script>

<template>
  <div ref="root" class="rl-slide-panel-stack">
    <!--
      Only present when it has something to do, so it never stands between
      the user and the page in the default case.
    -->
    <div
      v-if="closeOnClickOutside && panels.length > 0"
      class="rl-slide-panel-stack__scrim"
      @click="onScrimClick"
    />

    <!--
      Keyed by id so a panel that stays open across a change keeps its
      element, and its scroll position with it: opening a third panel must
      not scroll the second back to the top.
    -->
    <TransitionGroup name="rl-slide-panel">
      <RlSlidePanel
        v-for="(panel, index) in panels"
        :key="panel.id"
        :title="panel.title"
        :size="panel.size"
        :title-hidden="panel.titleHidden"
        :closable="panel.closable ?? true"
        :offset="offsets[index]"
        :style="{ zIndex: panels.length - index }"
        @close="emit('close', panel.id)"
      >
        <!--
          Both spellings, because Vue does not fold one into the other for a
          slot: a consumer writing `#panel-actions` and one writing
          `#panelActions` reach different keys on `$slots`, and accepting only
          the camelCase name would silently render nothing for the other.
        -->
        <template v-if="$slots.panelActions || $slots['panel-actions']" #actions>
          <slot name="panelActions" :panel="panel" :index="index" />
          <slot name="panel-actions" :panel="panel" :index="index" />
        </template>

        <slot name="panel" :panel="panel" :index="index" />
      </RlSlidePanel>
    </TransitionGroup>
  </div>
</template>

<style scoped>
/*
 * The stack covers the page it sits over so its panels can be positioned
 * against it, and hands the pointer back everywhere it is not drawing, so
 * the page behind stays clickable.
 */
.rl-slide-panel-stack {
  position: absolute;
  inset: 0;
  pointer-events: none;
  /*
   * Above the shell's own regions, including its detail panel, so a panel
   * opened from the navigation covers what is already on screen rather than
   * sliding under it. Still below every real overlay: a menu or a modal
   * opened from inside a panel must paint above the panel it came from.
   */
  z-index: var(--rl-z-shell-flyout);
  /*
   * Clips a panel to the stack while it travels, so a panel sliding in is
   * hidden until it crosses the stack's own left edge. That is what makes it
   * read as coming out from under whatever is to the left, the sidebar it
   * was opened from, rather than flying in over the top of it.
   */
  overflow: hidden;
}

.rl-slide-panel-stack :deep(.rl-slide-panel) { pointer-events: auto; }

.rl-slide-panel-stack__scrim {
  position: absolute;
  inset: 0;
  pointer-events: auto;
}

/*
 * Slides out from its own left edge, which is where it came from: a panel
 * opened by the sidebar emerges from under the sidebar, and one opened by a
 * row in the panel before it emerges from under that panel.
 *
 * Transform only, with no fade. A panel that emerges from behind something
 * is not translucent on the way out, and fading it makes it read as
 * appearing on top rather than sliding out. The clip on the stack above is
 * what hides the part that has not arrived yet.
 *
 * `ease-out` rather than `ease`, because the panel is arriving from behind
 * something rather than starting from rest in view: it should leave the edge
 * at speed and settle, not creep out and then rush.
 */
.rl-slide-panel-enter-active,
.rl-slide-panel-leave-active {
  transition: transform var(--rl-duration-base) ease-out;
}

.rl-slide-panel-enter-from,
.rl-slide-panel-leave-to {
  transform: translateX(-100%);
}

/*
 * Each panel sits under the one before it, so a panel opening emerges from
 * behind its parent rather than crossing over it. Without this they share a
 * layer, document order decides, and the newest panel is painted on top: it
 * then reads as flying in over the panel that opened it.
 *
 * The depth itself is set inline by the stack, because it is the panel's
 * position in the array and only the stack knows that. This rule just clears
 * the fixed layer the panel gives itself for standalone use, so the inline
 * value is what applies.
 */
.rl-slide-panel-stack :deep(.rl-slide-panel) { z-index: auto; }

/*
 * A leaving panel is taken out of the flow so the ones still open do not
 * wait for it, and drops under all of them while it goes.
 */
.rl-slide-panel-leave-active { z-index: 0 !important; }

/*
 * No travel for a reader who asked for less motion, so the panel fades in
 * place instead. The fade is the one case where opacity is right: nothing is
 * sliding, so there is no emergence to contradict.
 */
@media (prefers-reduced-motion: reduce) {
  .rl-slide-panel-enter-active,
  .rl-slide-panel-leave-active {
    transition: opacity var(--rl-duration-fast) var(--rl-ease);
  }

  .rl-slide-panel-enter-from,
  .rl-slide-panel-leave-to {
    transform: none;
    opacity: 0;
  }
}

/*
 * On a phone the stack is one column at a time: two 300px panels and a page
 * do not fit, so the innermost covers the rest and the ones behind it are
 * reached by closing it.
 */
@media (max-width: 767px) {
  .rl-slide-panel-stack :deep(.rl-slide-panel) {
    left: 0 !important;
    width: 100%;
  }

  .rl-slide-panel-stack :deep(.rl-slide-panel:not(:last-child)) {
    display: none;
  }
}
</style>
