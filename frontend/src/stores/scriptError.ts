import { defineStore } from 'pinia'
import { ref } from 'vue'

import type { ScriptError } from '../types/scriptError'

/**
 * Holds the most recent script-error envelope so a single dialog mounted
 * in App.vue can render it. Latest failure replaces previous: fixing one
 * broken script and triggering another should never leave a stale dialog
 * up.
 *
 * The triggering element is captured so focus can be restored on close —
 * essential for keyboard users invoking actions from the sidebar.
 */
export const useScriptErrorStore = defineStore('scriptError', () => {
  const current = ref<ScriptError | null>(null)
  const triggeringEl = ref<HTMLElement | null>(null)

  function show(err: ScriptError, fromEl?: HTMLElement | null): void {
    current.value = err
    triggeringEl.value = fromEl ?? null
  }

  function dismiss(): void {
    current.value = null
    // List actions can detach the trigger before dismiss runs (the
    // optimistic row-removal in useListActions.executeAction unmounts
    // the action header alongside the focused row). Calling .focus() on
    // a detached node silently no-ops and focus falls to <body>; the
    // contains() check makes that explicit.
    if (triggeringEl.value && document.contains(triggeringEl.value)) {
      triggeringEl.value.focus()
    }
    triggeringEl.value = null
  }

  return { current, show, dismiss }
})
