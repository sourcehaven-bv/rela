/**
 * Singleton confirm-dialog composable.
 *
 * One `<ConfirmModal>` is mounted in `App.vue` (via `useConfirmHost()`) and
 * bound to the module-level reactive `state` defined here. Anywhere in the
 * app, callers do:
 *
 *   const { confirm } = useConfirm()
 *   const ok = await confirm({ title, message, confirmLabel, danger })
 *   if (ok) doDestructiveThing()
 *
 * Callers MUST branch on the boolean. If the app shell unmounts (e.g. the
 * whole tab teardown) while a confirmation is pending, the pending promise
 * resolves to `false` so callers don't hang forever — that means an
 * `await confirm(); doThing()` pattern that ignores the boolean would
 * silently run `doThing()` after unmount. Don't write that.
 *
 * Concurrent calls share the in-flight promise: if a second `confirm()`
 * happens while one is open, both callers receive the same user decision.
 *
 * For confirmations that gate an async action (e.g. a network delete) and
 * want a "busy" state on the modal while the action runs, pass `onConfirm`:
 * the composable sets `busy = true`, awaits the callback, then resolves
 * `true` and closes. If the callback throws, the modal stays open with
 * `busy = false` so the user can retry or cancel — and the error is
 * rethrown so the caller's outer catch can surface it.
 */

import { onBeforeUnmount, reactive, readonly, type DeepReadonly } from 'vue'

export interface ConfirmOptions {
  title: string
  message?: string
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
  /**
   * Optional async action to run while the modal shows a "busy" state.
   * If it throws, the modal stays open (busy cleared) so the user can retry,
   * and the error is rethrown to the caller.
   */
  onConfirm?: () => Promise<void>
}

interface ConfirmState {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  cancelLabel: string
  danger: boolean
  busy: boolean
}

const DEFAULT_STATE: ConfirmState = {
  open: false,
  title: '',
  message: '',
  confirmLabel: 'Confirm',
  cancelLabel: 'Cancel',
  danger: false,
  busy: false,
}

const state = reactive<ConfirmState>({ ...DEFAULT_STATE })

let pendingPromise: Promise<boolean> | null = null
let pendingResolve: ((value: boolean) => void) | null = null
let pendingOnConfirm: (() => Promise<void>) | null = null
let hostMounted = false

function settle(value: boolean): void {
  const resolve = pendingResolve
  pendingPromise = null
  pendingResolve = null
  pendingOnConfirm = null
  state.open = false
  state.busy = false
  resolve?.(value)
}

async function onConfirmEvent(): Promise<void> {
  if (!pendingResolve) return
  const cb = pendingOnConfirm
  if (!cb) {
    settle(true)
    return
  }
  state.busy = true
  try {
    await cb()
    settle(true)
  } catch (err) {
    // Keep the modal open so the user can retry. Callers surface the error
    // via their own toast / logging — we just re-throw.
    state.busy = false
    throw err
  }
}

function onCancelEvent(): void {
  if (state.busy) return
  if (!pendingResolve) return
  settle(false)
}

function confirm(options: ConfirmOptions): Promise<boolean> {
  if (!hostMounted) {
    // Without a mounted host the modal can never appear and the returned
    // promise would hang forever. Surface the misuse clearly.
    if (typeof console !== 'undefined') {
      console.error(
        'useConfirm: no <ConfirmModal> host is mounted. Mount via useConfirmHost in App.vue.',
      )
    }
    return Promise.resolve(false)
  }
  if (pendingPromise) {
    // Concurrent call: return the in-flight promise. Both callers observe
    // the same user decision.
    return pendingPromise
  }

  pendingOnConfirm = options.onConfirm ?? null
  state.title = options.title
  state.message = options.message ?? ''
  state.confirmLabel = options.confirmLabel ?? DEFAULT_STATE.confirmLabel
  state.cancelLabel = options.cancelLabel ?? DEFAULT_STATE.cancelLabel
  state.danger = options.danger ?? false
  state.busy = false
  state.open = true

  pendingPromise = new Promise<boolean>((resolve) => {
    pendingResolve = resolve
  })
  return pendingPromise
}

/**
 * Caller-side: get the `confirm` function to invoke from anywhere.
 * Safe to call outside a component setup (it only returns a function).
 */
export function useConfirm(): { confirm: (options: ConfirmOptions) => Promise<boolean> } {
  return { confirm }
}

/**
 * Host-side: wires the singleton state and event handlers for App.vue's
 * `<ConfirmModal>`. Resolves any pending promise to `false` on app
 * unmount so callers don't hang. Must be called inside a component setup.
 *
 * Throws if a host is already mounted — the singleton must have exactly
 * one host. Mount it once at App.vue.
 */
export function useConfirmHost(): {
  state: DeepReadonly<ConfirmState>
  onConfirmEvent: () => Promise<void>
  onCancelEvent: () => void
} {
  if (hostMounted) {
    throw new Error(
      'useConfirmHost: a host is already mounted. Mount <ConfirmModal> only once at the App root.',
    )
  }
  hostMounted = true

  onBeforeUnmount(() => {
    hostMounted = false
    if (pendingResolve) settle(false)
  })

  return { state: readonly(state), onConfirmEvent, onCancelEvent }
}

/**
 * Wrap an async action with toast-on-error and rethrow, for use as `onConfirm`.
 * Lifts the repetitive try/uiStore.error/throw pattern at every confirm call site
 * into one place. The rethrow is required by useConfirm's contract — without it
 * the modal would close on a failed action.
 *
 * Intended usage:
 *
 *   await confirm({
 *     title: 'Delete?',
 *     onConfirm: withConfirmError(
 *       () => entitiesStore.remove(type, id),
 *       'Failed to delete entity',
 *       uiStore,
 *     ),
 *   })
 */
export function withConfirmError(
  action: () => Promise<unknown>,
  errorMessage: string,
  uiStore: { error: (msg: string) => void },
): () => Promise<void> {
  return async () => {
    try {
      await action()
    } catch (err) {
      uiStore.error(errorMessage)
      console.error(err)
      throw err
    }
  }
}

/**
 * Test-only: reset module state between tests. Mirrors the pattern in
 * modalStack.ts.
 */
export function _resetConfirmForTest(): void {
  pendingPromise = null
  pendingResolve = null
  pendingOnConfirm = null
  hostMounted = false
  Object.assign(state, DEFAULT_STATE)
}
