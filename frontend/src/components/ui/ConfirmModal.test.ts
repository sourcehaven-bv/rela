import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import ConfirmModal from './ConfirmModal.vue'

// The modal uses <Teleport to="body">, so mounted DOM lives on document.body,
// not inside wrapper.element. Tests query the real DOM and dispatch native
// events; assertions on emits still go through the wrapper.

describe('ConfirmModal', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  function factory(
    props: Record<string, unknown> = {},
    slots: Record<string, string> = {}
  ): VueWrapper {
    return mount(ConfirmModal, {
      props: {
        open: true,
        title: 'Test',
        ...props,
      },
      slots,
      attachTo: document.body,
    })
  }

  function overlay(): HTMLElement {
    const el = document.querySelector<HTMLElement>('.modal-overlay')
    if (!el) throw new Error('modal-overlay not in DOM')
    return el
  }

  function buttons(): HTMLButtonElement[] {
    return Array.from(
      document.querySelectorAll<HTMLButtonElement>('.modal-actions button')
    )
  }

  describe('rendering', () => {
    it('does not render when closed', () => {
      factory({ open: false })
      expect(document.querySelector('.modal-overlay')).toBeNull()
    })

    it('renders when open', () => {
      factory({ open: true, title: 'Delete Entity?', message: 'Are you sure?' })
      expect(document.querySelector('.modal-overlay')).not.toBeNull()
      expect(document.querySelector('h3')?.textContent).toBe('Delete Entity?')
      expect(document.querySelector('p')?.textContent).toBe('Are you sure?')
    })

    it('renders default slot content in place of message', () => {
      factory(
        { open: true, title: 'T', message: 'fallback' },
        { default: '<strong>slot content</strong>' }
      )
      expect(document.querySelector('p strong')?.textContent).toBe('slot content')
    })

    it('omits paragraph when no message and no slot', () => {
      factory({ open: true, title: 'T' })
      expect(document.querySelector('.modal p')).toBeNull()
    })

    it('uses default labels', () => {
      factory()
      const b = buttons()
      expect(b[0].textContent?.trim()).toBe('Cancel')
      expect(b[1].textContent?.trim()).toBe('Confirm')
    })

    it('uses custom labels', () => {
      factory({ confirmLabel: 'Delete', cancelLabel: 'Keep' })
      const b = buttons()
      expect(b[0].textContent?.trim()).toBe('Keep')
      expect(b[1].textContent?.trim()).toBe('Delete')
    })

    it('applies btn-danger class when danger=true', () => {
      factory({ danger: true })
      const [, confirmButton] = buttons()
      expect(confirmButton.classList.contains('btn-danger')).toBe(true)
      expect(confirmButton.classList.contains('btn-primary')).toBe(false)
    })

    it('applies btn-primary class when danger=false', () => {
      factory({ danger: false })
      const [, confirmButton] = buttons()
      expect(confirmButton.classList.contains('btn-primary')).toBe(true)
      expect(confirmButton.classList.contains('btn-danger')).toBe(false)
    })
  })

  describe('focus behavior', () => {
    it('focuses Cancel button on open', async () => {
      const wrapper = mount(ConfirmModal, {
        props: { open: false, title: 'T' },
        attachTo: document.body,
      })
      await wrapper.setProps({ open: true })
      await flushPromises()

      const [cancelButton] = buttons()
      expect(document.activeElement).toBe(cancelButton)
      wrapper.unmount()
    })

    it('restores previously focused element on close', async () => {
      const trigger = document.createElement('button')
      trigger.textContent = 'Open'
      document.body.appendChild(trigger)
      trigger.focus()
      expect(document.activeElement).toBe(trigger)

      const wrapper = mount(ConfirmModal, {
        props: { open: false, title: 'T' },
        attachTo: document.body,
      })

      await wrapper.setProps({ open: true })
      await flushPromises()
      const [cancelButton] = buttons()
      expect(document.activeElement).toBe(cancelButton)

      await wrapper.setProps({ open: false })
      await flushPromises()
      expect(document.activeElement).toBe(trigger)

      wrapper.unmount()
    })
  })

  describe('emits', () => {
    it('emits confirm when confirm button clicked', () => {
      const wrapper = factory()
      buttons()[1].click()
      expect(wrapper.emitted('confirm')).toHaveLength(1)
    })

    it('emits cancel when cancel button clicked', () => {
      const wrapper = factory()
      buttons()[0].click()
      expect(wrapper.emitted('cancel')).toHaveLength(1)
    })

    it('emits cancel on overlay click', () => {
      const wrapper = factory()
      // Click with target === currentTarget (bare overlay click, not bubbled)
      overlay().dispatchEvent(new MouseEvent('click', { bubbles: true }))
      expect(wrapper.emitted('cancel')).toHaveLength(1)
    })

    it('does not emit cancel when clicking inside modal content', () => {
      const wrapper = factory()
      const modal = document.querySelector<HTMLElement>('.modal')!
      modal.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      expect(wrapper.emitted('cancel')).toBeUndefined()
    })

    it('emits cancel on Escape keydown', () => {
      const wrapper = factory()
      overlay().dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })
      )
      expect(wrapper.emitted('cancel')).toHaveLength(1)
    })

    it('stops propagation of Escape keydown', () => {
      factory()
      const event = new KeyboardEvent('keydown', {
        key: 'Escape',
        bubbles: true,
        cancelable: true,
      })
      const stopSpy = vi.spyOn(event, 'stopPropagation')
      overlay().dispatchEvent(event)
      expect(stopSpy).toHaveBeenCalled()
    })

    it('does not emit cancel for non-Escape keys', () => {
      const wrapper = factory()
      overlay().dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })
      )
      expect(wrapper.emitted('cancel')).toBeUndefined()
    })
  })

  describe('busy state', () => {
    it('disables both buttons when busy=true', () => {
      factory({ busy: true })
      const b = buttons()
      expect(b[0].disabled).toBe(true)
      expect(b[1].disabled).toBe(true)
    })

    it('shows loading label on confirm button when busy', () => {
      factory({ busy: true, confirmLabel: 'Delete' })
      const [, confirmButton] = buttons()
      expect(confirmButton.textContent?.trim()).toBe('Delete\u2026')
    })

    it('does not emit cancel on overlay click while busy', () => {
      const wrapper = factory({ busy: true })
      overlay().dispatchEvent(new MouseEvent('click', { bubbles: true }))
      expect(wrapper.emitted('cancel')).toBeUndefined()
    })

    it('does not emit cancel on Escape while busy', () => {
      const wrapper = factory({ busy: true })
      overlay().dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })
      )
      expect(wrapper.emitted('cancel')).toBeUndefined()
    })
  })
})
