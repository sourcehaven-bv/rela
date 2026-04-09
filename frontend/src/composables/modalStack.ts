/**
 * Shared modal-stack registry.
 *
 * Any component that behaves as a modal / dialog (Teleported overlay,
 * dropdown menu, confirm dialog, etc.) should call `registerModal` when it
 * opens and unregister when it closes. Keyboard-shortcut handlers elsewhere
 * in the app can call `isAnyModalOpen()` to avoid firing shortcuts while any
 * modal is present.
 *
 * This replaces a fragile `document.querySelector('.modal-overlay')` guard
 * that relied on CSS-class collision to detect modal presence and silently
 * disabled shortcuts whenever any unrelated modal happened to use the same
 * class. The stack is an explicit, testable registry.
 */

import { onBeforeUnmount, watch, type Ref } from 'vue'

const openModals = new Set<symbol>()

export function registerModal(id: symbol): void {
  openModals.add(id)
}

export function unregisterModal(id: symbol): void {
  openModals.delete(id)
}

export function isAnyModalOpen(): boolean {
  return openModals.size > 0
}

/**
 * Convenience composable: wires a reactive `open` ref to the modal stack
 * automatically, including cleanup on unmount.
 */
export function useModalStack(open: Ref<boolean>): void {
  const id = Symbol('modal')
  watch(
    open,
    (isOpen) => {
      if (isOpen) {
        registerModal(id)
      } else {
        unregisterModal(id)
      }
    },
    { immediate: true }
  )
  onBeforeUnmount(() => {
    unregisterModal(id)
  })
}

/**
 * Test-only: clear the stack. Used between tests to prevent leakage.
 */
export function _resetModalStack(): void {
  openModals.clear()
}
