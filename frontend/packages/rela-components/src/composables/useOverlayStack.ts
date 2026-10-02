/**
 * Shared registry of open overlays.
 *
 * Without it, two open overlays both listen for Escape and both close, and
 * each one restores the body scroll the other still needs. Registering makes
 * the stack explicit: only the topmost overlay handles a key, and the scroll
 * lock is released only when the last overlay closes.
 */
import { onBeforeUnmount, ref } from 'vue'

const stack = ref<symbol[]>([])
let previousOverflow: string | null = null

/** True when `id` is the overlay on top, so it owns the keyboard. */
export function isTopmost(id: symbol): boolean {
  return stack.value[stack.value.length - 1] === id
}

export function overlayDepth(): number {
  return stack.value.length
}

function lockScroll() {
  if (typeof document === 'undefined' || previousOverflow !== null) return
  previousOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
}

function unlockScroll() {
  if (typeof document === 'undefined' || previousOverflow === null) return
  document.body.style.overflow = previousOverflow
  previousOverflow = null
}

/**
 * Registers one overlay. `push` on open and `pop` on close; both are safe to
 * call twice. `scrollLock` is off for overlays such as a menu, which should
 * not stop the page scrolling underneath.
 */
export function useOverlayStack(options: { scrollLock?: boolean } = {}) {
  const { scrollLock = true } = options
  const id = Symbol('rl-overlay')

  function push() {
    if (stack.value.includes(id)) return
    stack.value = [...stack.value, id]
    if (scrollLock) lockScroll()
  }

  function pop() {
    if (!stack.value.includes(id)) return
    stack.value = stack.value.filter((entry) => entry !== id)
    // Another overlay may still need the lock, so only the last one frees it.
    if (stack.value.length === 0) unlockScroll()
  }

  onBeforeUnmount(pop)

  return { id, push, pop, isTop: () => isTopmost(id) }
}

/**
 * Returns focus to wherever it was before an overlay opened (WCAG 3.2.1).
 * Stores the element on open rather than on close, because by then the
 * overlay itself holds focus.
 */
export function useFocusRestore() {
  let previous: HTMLElement | null = null

  return {
    capture() {
      previous = typeof document === 'undefined' ? null : (document.activeElement as HTMLElement | null)
    },
    restore() {
      /*
       * The element may have been unmounted while the overlay was open, for
       * instance when a row is removed optimistically by the action that
       * opened the dialog. Calling focus() on a detached node does nothing
       * and leaves focus on <body>, which is the outcome this exists to
       * prevent. Check the node is still in the document before focusing it.
       */
      if (previous && typeof document !== 'undefined' && document.contains(previous)) {
        previous.focus?.()
      }
      previous = null
    },
  }
}

/**
 * Selector for the things a Tab press can reach. Used by every overlay that
 * has to keep focus inside itself.
 */
export const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

/** Wraps Tab from the last control to the first, and Shift+Tab back. */
export function trapTab(container: HTMLElement | null, event: KeyboardEvent) {
  const items = container?.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)
  if (!items?.length) return

  const first = items[0]
  const last = items[items.length - 1]
  const active = document.activeElement

  if (event.shiftKey && active === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && active === last) {
    event.preventDefault()
    first.focus()
  }
}
