<script setup lang="ts">
/**
 * Dropdown menu, the panel behind a "..." button.
 *
 * Follows the ARIA menu button pattern: the trigger owns `aria-expanded`, the
 * panel is a `menu` of `menuitem`s, and arrow keys move between items while
 * Tab leaves the menu entirely. Home and End jump to the ends, which matters
 * once a menu is longer than a few entries.
 */
import { computed, nextTick, ref } from 'vue'
import { useOverlayStack } from '../../composables/useOverlayStack'
import { useAnchoredPanel } from '../../composables/useAnchoredPanel'

const props = withDefaults(
  defineProps<{
    /** Which edge of the trigger the panel lines up with. */
    align?: 'start' | 'end'
    /** Preferred side. The panel flips to the other one when that has no room. */
    placement?: 'bottom' | 'top'
  }>(),
  { align: 'end', placement: 'bottom' },
)

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)

/*
 * `align` and `placement` are the preference, not the outcome: a menu opened
 * near an edge flips and shifts to stay on screen.
 */
const { floatingStyles, containsTarget } = useAnchoredPanel(root, panel, {
  placement: computed(() => `${props.placement}-${props.align}` as const),
})

// No scroll lock: a menu should not stop the page behind it scrolling.
const { push, pop } = useOverlayStack({ scrollLock: false })

/*
 * Both spellings of disabled. A native `<button>` carries the attribute; a
 * link cannot, because `disabled` means nothing on an `<a>`, so a disabled
 * link row reports `aria-disabled` instead. Checking only the attribute let
 * arrow keys land on a row that cannot be used.
 */
function items(): HTMLElement[] {
  return Array.from(panel.value?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? []).filter(
    (item) =>
      !item.hasAttribute('disabled') && item.getAttribute('aria-disabled') !== 'true',
  )
}

async function show(focusIndex = 0) {
  open.value = true
  push()
  await nextTick()
  const list = items()
  list[focusIndex === -1 ? list.length - 1 : focusIndex]?.focus()
}

function hide(restoreFocus = true) {
  if (!open.value) return
  open.value = false
  pop()
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
  'aria-haspopup': 'menu'
  'aria-expanded': boolean
  onKeydown: (event: KeyboardEvent) => void
}

const triggerAttrs = computed<TriggerAttrs>(() => ({
  'aria-haspopup': 'menu',
  'aria-expanded': open.value,
  onKeydown: onTriggerKeydown,
}))

function onTriggerKeydown(event: KeyboardEvent) {
  // Arrow keys open the menu and land on an end item, as the pattern expects.
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    show(0)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    show(-1)
  }
}

function onPanelKeydown(event: KeyboardEvent) {
  const list = items()
  if (!list.length) return
  const current = list.indexOf(document.activeElement as HTMLElement)

  switch (event.key) {
    case 'ArrowDown':
      event.preventDefault()
      list[(current + 1) % list.length].focus()
      break
    case 'ArrowUp':
      event.preventDefault()
      list[(current - 1 + list.length) % list.length].focus()
      break
    case 'Home':
      event.preventDefault()
      list[0].focus()
      break
    case 'End':
      event.preventDefault()
      list[list.length - 1].focus()
      break
    case 'Escape':
      event.stopPropagation()
      hide()
      break
    case 'Tab':
      // Tab means "leave", so the menu closes rather than trapping focus.
      hide(false)
      break
  }
}

function onFocusout(event: FocusEvent) {
  // The panel is teleported, so it is not inside `root` — ask the composable,
  // which knows about both.
  if (!containsTarget(root.value, event.relatedTarget as Node | null)) hide(false)
}

defineExpose({ close: () => hide() })
</script>

<template>
  <div ref="root" class="rl-menu" @focusout="onFocusout">
    <!--
      The trigger is supplied by the caller so any button can open a menu.
      The binding carries the ARIA state, so a caller cannot forget it.
    -->
    <slot
      name="trigger"
      :open="open"
      :toggle="() => (open ? hide() : show())"
      :attrs="triggerAttrs"
    />

    <!--
      Teleported so an ancestor with `overflow: hidden` — a scrolling table,
      a card — cannot clip the panel. `focusout` still fires on the root
      because focus moves are tracked by the focus tree, not the DOM tree.
    -->
    <Teleport to="body">
      <div
        v-if="open"
        ref="panel"
        class="rl-panel rl-panel--menu"
        :style="floatingStyles"
        role="menu"
        @keydown="onPanelKeydown"
        @focusout="onFocusout"
        @click="hide()"
      >
        <slot />
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.rl-menu {
  position: relative;
  display: inline-flex;
}

/* The panel itself is styled in styles/panel.css: it is teleported, so a
   scoped rule here would not reach it. */
</style>
