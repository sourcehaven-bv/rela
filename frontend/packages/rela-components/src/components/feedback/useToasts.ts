/**
 * Queue behind RlToastHost.
 *
 * Module-level state, so any component can raise a toast without the host
 * being passed down to it. That is the whole point of a toast: the code that
 * knows something happened is rarely the code that owns the corner it shows in.
 */
import { ref } from 'vue'
import type { MessageTone, ToastMessage } from './types'

const toasts = ref<ToastMessage[]>([])
let counter = 0

export function useToasts() {
  function show(message: Omit<ToastMessage, 'id'>): string {
    const id = `toast-${(counter += 1)}`
    toasts.value = [...toasts.value, { id, ...message }]
    return id
  }

  function dismiss(id: string) {
    toasts.value = toasts.value.filter((toast) => toast.id !== id)
  }

  /** Shorthands, so a call site reads as what happened. */
  const tone = (value: MessageTone) => (title: string, description?: string) =>
    show({ title, description, tone: value })

  return {
    toasts,
    show,
    dismiss,
    clear: () => (toasts.value = []),
    info: tone('info'),
    success: tone('success'),
    warning: tone('warning'),
    danger: tone('danger'),
  }
}
