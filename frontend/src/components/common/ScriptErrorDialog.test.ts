import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import ScriptErrorDialog from './ScriptErrorDialog.vue'
import { useScriptErrorStore } from '../../stores/scriptError'
import { isAnyModalOpen, _resetModalStack } from '@/composables/modalStack'
import type { ScriptError } from '../../types/scriptError'

/**
 * The dialog moved onto the shared `RlModal` (rela-components). These tests
 * pin the three things that migration could plausibly have broken, all of
 * which live at the seam between the two projects rather than inside either.
 *
 * `RlModal` teleports to `<body>`, so the DOM assertions query the document
 * rather than the wrapper.
 */

const ERROR: ScriptError = {
  error: 'script_error',
  script: { surface: 'action', path: 'scripts/thing.lua' },
  lua: { message: 'attempt to index a nil value' },
}

describe('ScriptErrorDialog', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    _resetModalStack()
    document.body.innerHTML = ''
  })

  afterEach(() => {
    _resetModalStack()
    document.body.innerHTML = ''
  })

  it('renders nothing until the store holds an error', () => {
    mount(ScriptErrorDialog, { attachTo: document.body })

    expect(document.querySelector('[role="alertdialog"]')).toBeNull()
  })

  it('renders the error as an alertdialog once the store has one', async () => {
    const wrapper = mount(ScriptErrorDialog, { attachTo: document.body })
    const store = useScriptErrorStore()

    store.show(ERROR)
    await wrapper.vm.$nextTick()

    const dialog = document.querySelector('[role="alertdialog"]')
    expect(dialog).not.toBeNull()
    expect(dialog?.getAttribute('aria-modal')).toBe('true')
    expect(document.body.textContent).toContain('attempt to index a nil value')
  })

  /**
   * The regression this guards: rela's modal stack is a DIFFERENT registry
   * from the library's overlay stack. `isAnyModalOpen()` is what suppresses
   * global keyboard shortcuts, so a dialog that registers only with the
   * library's stack leaves shortcuts firing underneath an open error report.
   */
  it('registers with rela’s modal stack, not just the library’s', async () => {
    const wrapper = mount(ScriptErrorDialog, { attachTo: document.body })
    const store = useScriptErrorStore()
    expect(isAnyModalOpen()).toBe(false)

    store.show(ERROR)
    await wrapper.vm.$nextTick()
    expect(isAnyModalOpen()).toBe(true)

    store.dismiss()
    await wrapper.vm.$nextTick()
    expect(isAnyModalOpen()).toBe(false)
  })

  it('dismisses through the store when the modal asks to close', async () => {
    const wrapper = mount(ScriptErrorDialog, { attachTo: document.body })
    const store = useScriptErrorStore()

    store.show(ERROR)
    await wrapper.vm.$nextTick()

    // The close affordance RlModal renders in its header.
    const close = document.querySelector<HTMLButtonElement>(
      '[role="alertdialog"] button'
    )
    close?.click()
    await wrapper.vm.$nextTick()

    expect(store.current).toBeNull()
    expect(document.querySelector('[role="alertdialog"]')).toBeNull()
  })

  /**
   * Focus restore stays the store's job, because the element that opened the
   * dialog is often detached by the time it closes (a list action removes its
   * row optimistically). The store checks `document.contains` first; the
   * library's `useFocusRestore` does not.
   */
  it('leaves focus restoration to the store', async () => {
    const trigger = document.createElement('button')
    document.body.appendChild(trigger)
    trigger.focus()

    const wrapper = mount(ScriptErrorDialog, { attachTo: document.body })
    const store = useScriptErrorStore()

    store.show(ERROR, trigger)
    await wrapper.vm.$nextTick()

    store.dismiss()
    await wrapper.vm.$nextTick()

    expect(document.activeElement).toBe(trigger)
  })
})
