import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ConfirmModal from './ConfirmModal.vue'
import { _resetModalStack, isAnyModalOpen } from '@/composables/modalStack'

/**
 * ConfirmModal is an adapter over `RlConfirmDialog` (rela-components). These
 * assert the CONTRACT `useConfirm()` depends on — what renders, what is
 * emitted, where focus goes — rather than the markup underneath, which now
 * belongs to the library and is its to change.
 *
 * The dialog teleports to `<body>`, so DOM queries go through the document
 * and assertions on emits go through the wrapper.
 */

describe('ConfirmModal', () => {
  beforeEach(() => {
    _resetModalStack()
    document.body.innerHTML = ''
  })

  afterEach(() => {
    _resetModalStack()
    document.body.innerHTML = ''
    vi.useRealTimers()
  })

  // Annotated so `setProps` keeps the component's prop types; an inferred
  // return from a generic factory widens them away.
  type Wrapper = ReturnType<typeof mount<typeof ConfirmModal>>

  function factory(props: Record<string, unknown> = {}): Wrapper {
    return mount(ConfirmModal, {
      props: {
        open: true,
        title: 'Test',
        ...props,
      },
      attachTo: document.body,
    })
  }

  function dialog(): HTMLElement {
    const el = document.querySelector<HTMLElement>('[role="alertdialog"]')
    if (!el) throw new Error('confirm dialog not in DOM')
    return el
  }

  /** [cancel, confirm] — DOM order, which is also the tab order. */
  function buttons(): HTMLButtonElement[] {
    return Array.from(dialog().querySelectorAll<HTMLButtonElement>('button'))
  }

  describe('rendering', () => {
    it('does not render when closed', () => {
      factory({ open: false })
      expect(document.querySelector('[role="alertdialog"]')).toBeNull()
    })

    it('renders the title and message when open', () => {
      factory({ open: true, title: 'Delete Entity?', message: 'Are you sure?' })

      expect(dialog().getAttribute('aria-modal')).toBe('true')
      expect(dialog().textContent).toContain('Delete Entity?')
      expect(dialog().textContent).toContain('Are you sure?')
    })

    it('omits the message paragraph when there is no message', () => {
      factory({ open: true, title: 'Just a title' })
      expect(dialog().querySelector('p')).toBeNull()
    })

    /**
     * A caller may present a list ("these fields will be cleared"); without
     * `pre-line` it collapses into one run-on paragraph. The rule is scoped
     * from this component onto the library's paragraph, so it only applies if
     * the class actually reaches through — which is what this really pins.
     */
    it('marks the message so authored line breaks survive', () => {
      factory({ open: true, message: 'One\nTwo' })

      const paragraph = dialog().querySelector('.rl-confirm-dialog__description')
      expect(paragraph).not.toBeNull()
      expect(document.querySelector('.rela-confirm')).not.toBeNull()
    })

    it('uses default labels', () => {
      factory()
      expect(buttons().map((b) => b.textContent?.trim())).toEqual([
        'Cancel',
        'Confirm',
      ])
    })

    it('uses custom labels', () => {
      factory({ confirmLabel: 'Delete', cancelLabel: 'Keep' })
      expect(buttons().map((b) => b.textContent?.trim())).toEqual([
        'Keep',
        'Delete',
      ])
    })

    /**
     * `danger` used to pick a `btn-danger` class. It now selects the library's
     * danger TONE, so the assertion is that the two states differ and that the
     * dialog says which it is — not the name of a class rela no longer owns.
     */
    it('distinguishes a destructive confirm from a neutral one', () => {
      const dangerous = factory({ danger: true })
      const dangerClasses = buttons()[1].className
      dangerous.unmount()
      document.body.innerHTML = ''

      factory({ danger: false })
      expect(buttons()[1].className).not.toBe(dangerClasses)
    })
  })

  describe('focus behavior', () => {
    /**
     * Cancel, not Confirm: a stray Enter or Space on an alertdialog must not
     * perform the destructive action. Mirrors `window.confirm`.
     */
    it('focuses Cancel on open', async () => {
      const wrapper: Wrapper = mount(ConfirmModal, {
        props: { open: false, title: 'T' },
        attachTo: document.body,
      })
      await wrapper.setProps({ open: true })
      await flushPromises()

      expect(document.activeElement).toBe(buttons()[0])
      wrapper.unmount()
    })

    it('restores the previously focused element on close', async () => {
      const trigger = document.createElement('button')
      document.body.appendChild(trigger)
      trigger.focus()

      const wrapper: Wrapper = mount(ConfirmModal, {
        props: { open: false, title: 'T' },
        attachTo: document.body,
      })

      await wrapper.setProps({ open: true })
      await flushPromises()
      expect(document.activeElement).toBe(buttons()[0])

      await wrapper.setProps({ open: false })
      await flushPromises()
      expect(document.activeElement).toBe(trigger)

      wrapper.unmount()
    })
  })

  describe('emits', () => {
    it('emits confirm when the confirm button is clicked', () => {
      const wrapper = factory()
      buttons()[1].click()
      expect(wrapper.emitted('confirm')).toHaveLength(1)
    })

    it('emits cancel when the cancel button is clicked', () => {
      const wrapper = factory()
      buttons()[0].click()
      expect(wrapper.emitted('cancel')).toHaveLength(1)
    })

    it('emits cancel on a scrim click', () => {
      const wrapper = factory()

      const scrim = document.querySelector<HTMLElement>('.rl-modal__scrim')!
      scrim.dispatchEvent(new MouseEvent('click', { bubbles: true }))

      expect(wrapper.emitted('cancel')).toHaveLength(1)
    })

    it('does not emit cancel when the click is inside the dialog', () => {
      const wrapper = factory()
      dialog().dispatchEvent(new MouseEvent('click', { bubbles: true }))
      expect(wrapper.emitted('cancel')).toBeUndefined()
    })

    /** Escape is handled on the document now, by the shared overlay stack. */
    it('emits cancel on Escape', () => {
      const wrapper = factory()

      document.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })
      )

      expect(wrapper.emitted('cancel')).toHaveLength(1)
    })

    it('does not emit cancel for other keys', () => {
      const wrapper = factory()

      document.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })
      )

      expect(wrapper.emitted('cancel')).toBeUndefined()
    })
  })

  describe('busy state', () => {
    /**
     * `aria-disabled`, not native `disabled`, on the primary action: native
     * disabled drops focus to `<body>` mid-interaction and strands a keyboard
     * user on the control they just pressed (RR-R5VL59).
     *
     * Cancel stays focusable and keeps its enabled appearance; the library
     * suppresses the action itself while `confirming`, which the behavioural
     * cases below assert. So this checks the confirm button's contract and
     * leaves "can you still cancel" to the emit tests, where it is observable.
     */
    it('marks the confirm action busy without stranding focus', () => {
      factory({ busy: true })
      const [, confirm] = buttons()

      expect(confirm.getAttribute('aria-disabled')).toBe('true')
      expect(confirm.getAttribute('aria-busy')).toBe('true')
    })

    it('does not emit confirm from a click while busy', () => {
      const wrapper = factory({ busy: true })
      buttons()[1].click()
      expect(wrapper.emitted('confirm')).toBeUndefined()
    })

    it('does not emit cancel from the cancel button while busy', () => {
      const wrapper = factory({ busy: true })
      buttons()[0].click()
      expect(wrapper.emitted('cancel')).toBeUndefined()
    })

    it('does not emit cancel on a scrim click while busy', () => {
      const wrapper = factory({ busy: true })

      const scrim = document.querySelector<HTMLElement>('.rl-modal__scrim')!
      scrim.dispatchEvent(new MouseEvent('click', { bubbles: true }))

      expect(wrapper.emitted('cancel')).toBeUndefined()
    })

    it('does not emit cancel on Escape while busy', () => {
      const wrapper = factory({ busy: true })

      document.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })
      )

      expect(wrapper.emitted('cancel')).toBeUndefined()
    })

    /**
     * The label swap is GATED, where this component used to swap instantly.
     * rela's own rule is "never wire an indicator straight to a boolean" — a
     * 40ms request should flash nothing at all — and the library applies the
     * same 500ms delay rela uses for an explicit action.
     */
    it('holds the resting label through a fast action', async () => {
      vi.useFakeTimers()
      const wrapper: Wrapper = mount(ConfirmModal, {
        props: { open: true, title: 'T', confirmLabel: 'Delete', busy: false },
        attachTo: document.body,
      })

      await wrapper.setProps({ busy: true })
      vi.advanceTimersByTime(100)
      await flushPromises()

      expect(buttons()[1].textContent?.trim()).toBe('Delete')
      wrapper.unmount()
    })
  })

  /**
   * rela's modal stack is a separate registry from the library's overlay
   * stack: `isAnyModalOpen()` is what suppresses global keyboard shortcuts.
   */
  it('registers with rela’s modal stack while open', async () => {
    const wrapper: Wrapper = mount(ConfirmModal, {
      props: { open: false, title: 'T' },
      attachTo: document.body,
    })
    expect(isAnyModalOpen()).toBe(false)

    await wrapper.setProps({ open: true })
    expect(isAnyModalOpen()).toBe(true)

    await wrapper.setProps({ open: false })
    expect(isAnyModalOpen()).toBe(false)
    wrapper.unmount()
  })
})
