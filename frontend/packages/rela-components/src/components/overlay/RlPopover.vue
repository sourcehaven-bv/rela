<script setup lang="ts">
/**
 * A panel anchored to the control that opened it, holding whatever the caller
 * puts in it.
 *
 * This is the non-menu half of what a floating panel can be. `RlMenu` is a
 * list of commands: it is a `menu` of `menuitem`s, arrow keys move between
 * them, and any click inside chooses something and so closes the panel. A
 * popover holds prose, links and mixed controls, where every one of those
 * behaviours is wrong — `menuitem` misdescribes a paragraph, and closing on
 * the first click inside dismisses the panel before a link in the body can
 * navigate.
 *
 * So the two differ in exactly three places, and share everything else:
 *
 *   - It is a `dialog`, named by `title`, rather than a `menu`.
 *   - Closing is explicit: Escape, a click outside, focus leaving, or the
 *     caller calling `close`. A click inside does nothing on its own.
 *   - Tab cycles within the panel rather than leaving it, because there is no
 *     single "the items" to step through and out of.
 *
 * It is not modal. The page keeps scrolling underneath and the rest of the
 * document stays reachable, which is why it registers with the overlay stack
 * without the scroll lock: the stack is what stops one Escape press closing
 * this panel and the dialog behind it together.
 */
import { computed, nextTick, ref } from 'vue'
import { FOCUSABLE_SELECTOR, trapTab, useOverlayStack } from '../../composables/useOverlayStack'
import { useAnchoredPanel } from '../../composables/useAnchoredPanel'

const props = withDefaults(
  defineProps<{
    /**
     * Names the panel for a screen reader. A dialog with no accessible name
     * is announced as an unlabelled group, so this is required rather than
     * optional.
     */
    title: string
    /** Which edge of the trigger the panel lines up with. */
    align?: 'start' | 'end'
    /** Preferred side. The panel flips to the other one when that has no room. */
    placement?: 'bottom' | 'top'
  }>(),
  { align: 'end', placement: 'bottom' },
)

const emit = defineEmits<{ open: []; close: [] }>()

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)

/*
 * `align` and `placement` are the preference, not the outcome: a panel opened
 * near an edge flips and shifts to stay on screen.
 */
const { floatingStyles, containsTarget } = useAnchoredPanel(root, panel, {
  placement: computed(() => `${props.placement}-${props.align}` as const),
})

// No scroll lock: a popover should not stop the page behind it scrolling.
const { push, pop, isTop } = useOverlayStack({ scrollLock: false })

async function show() {
  if (open.value) return
  open.value = true
  push()
  emit('open')
  await nextTick()
  /*
   * Focus the first control, falling back to the panel. Without this the
   * keyboard stays on the trigger, where Tab walks into the page behind the
   * open panel rather than into the panel itself.
   */
  const first = panel.value?.querySelector<HTMLElement>(FOCUSABLE_SELECTOR)
  ;(first ?? panel.value)?.focus()
}

function hide(restoreFocus = true) {
  if (!open.value) return
  open.value = false
  pop()
  emit('close')
  if (restoreFocus) root.value?.querySelector<HTMLElement>('[aria-haspopup]')?.focus()
}

/*
 * What the trigger has to carry, stated exactly.
 *
 * Both halves of this matter, and they pull in opposite directions. Left to
 * inference, `aria-haspopup` widens to `string`, which a native `<button>`
 * rejects because its own attribute is a union of specific values. Annotated
 * with Vue's `ButtonHTMLAttributes` instead, the object widens the other way —
 * to every optional native button attribute — and then a component trigger
 * rejects it, because a component's props are a closed set and native
 * attributes admit `Booleanish` where a prop wants a real boolean.
 *
 * An exact interface is what satisfies both: three keys and no more, so a
 * component accepts it, with the literal and the boolean stated, so a native
 * element accepts it too and the values are still checked here.
 */
interface TriggerAttrs {
  'aria-haspopup': 'dialog'
  'aria-expanded': boolean
  onKeydown: (event: KeyboardEvent) => void
}

const triggerAttrs = computed<TriggerAttrs>(() => ({
  'aria-haspopup': 'dialog',
  'aria-expanded': open.value,
  onKeydown: onTriggerKeydown,
}))

function onTriggerKeydown(event: KeyboardEvent) {
  // Down opens, matching the menu button pattern a trigger is expected to follow.
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    show()
  }
}

function onPanelKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    // Only the topmost overlay acts, so one press closes one panel.
    if (!isTop()) return
    event.stopPropagation()
    hide()
  } else if (event.key === 'Tab') {
    /*
     * Cycles rather than leaving. A menu closes on Tab because its content is
     * a list the user is stepping through; a popover's content is a small form
     * of its own, and tabbing out of it while it stays open puts focus on
     * things the panel is covering.
     */
    trapTab(panel.value, event)
  }
}

function onFocusout(event: FocusEvent) {
  // The panel is teleported, so it is not inside `root` — ask the composable,
  // which knows about both.
  if (!containsTarget(root.value, event.relatedTarget as Node | null)) hide(false)
}

/*
 * A click outside closes; a click inside does not. This is the behaviour a
 * menu cannot offer, and the reason a panel with a link in its body needs this
 * component: the panel has to survive the click that follows the link.
 */
function onScrimPointerdown(event: PointerEvent) {
  if (containsTarget(root.value, event.target as Node | null)) return
  hide(false)
}

defineExpose({
  open: () => show(),
  close: () => hide(),
  toggle: () => (open.value ? hide() : show()),
})
</script>

<template>
  <div ref="root" class="rl-popover" @focusout="onFocusout">
    <!--
      The trigger is supplied by the caller so any control can open a popover.
      The binding carries the ARIA state, so a caller cannot forget it.
    -->
    <slot
      name="trigger"
      :open="open"
      :toggle="() => (open ? hide() : show())"
      :attrs="triggerAttrs"
    />

    <Teleport to="body">
      <!--
        A transparent full-page layer catches the click that closes the panel.
        `pointerdown` rather than `click`, so the panel is already gone when
        the press completes; on `click` a press that started outside and
        released over moved content would close nothing.

        It only catches: it does not tint and does not swallow the event, so
        the control that was clicked still receives it. That is what keeps the
        popover non-modal — one click can dismiss this panel and act on
        something else, which is how a small anchored panel is expected to
        behave.
      -->
      <div v-if="open" class="rl-popover__catcher" @pointerdown="onScrimPointerdown" />

      <!--
        Teleported so an ancestor with `overflow: hidden` — a scrolling list,
        a sidebar footer — cannot clip the panel. `focusout` still fires on the
        root because focus moves are tracked by the focus tree, not the DOM tree.
      -->
      <div
        v-if="open"
        ref="panel"
        class="rl-panel rl-panel--popover"
        :style="floatingStyles"
        role="dialog"
        :aria-label="title"
        tabindex="-1"
        @keydown="onPanelKeydown"
        @focusout="onFocusout"
      >
        <slot :close="() => hide()" />
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.rl-popover {
  position: relative;
  display: inline-flex;
}

/*
 * Under the panel and over the page. Teleported like the panel, so this rule
 * reaches it only because the catcher is the component's own element rather
 * than slotted content.
 */
.rl-popover__catcher {
  position: fixed;
  inset: 0;
  z-index: calc(var(--rl-z-menu) - 1);
}

/* The panel itself is styled in styles/panel.css: it is teleported, so a
   scoped rule here would not reach it. */
</style>
