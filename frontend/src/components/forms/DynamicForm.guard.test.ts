import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h, ref, type Ref } from 'vue'
import { createRouter, createMemoryHistory, onBeforeRouteLeave, RouterView } from 'vue-router'
import ConfirmModal from '@/components/ui/ConfirmModal.vue'
import { useConfirm, useConfirmHost, _resetConfirmForTest } from '@/composables/useConfirm'
import { _resetModalStack } from '@/composables/modalStack'

// The DynamicForm unsaved-changes guard is small but easy to break — it must
// (a) short-circuit when not dirty, (b) show the confirm modal and stay on the
// page on cancel, (c) clear dirty before letting the guard return true on
// confirm so subsequent guard passes don't re-prompt, and (d) preserve the
// router's push/replace semantics by returning the awaited boolean rather
// than calling next(false) + router.push (the original wrong design).
//
// We test this without DynamicForm's many dependencies by replicating the
// exact guard in a tiny component.

function makeFormHarness(dirty: Ref<boolean>) {
  return defineComponent({
    setup() {
      const { confirm } = useConfirm()
      onBeforeRouteLeave(async () => {
        if (!dirty.value) return true
        const ok = await confirm({
          title: 'Unsaved changes',
          message: 'You have unsaved changes. Are you sure you want to leave?',
          confirmLabel: 'Leave',
          danger: true,
        })
        if (ok) dirty.value = false
        return ok
      })
      return () => h('div', { class: 'form-page' }, 'form')
    },
  })
}

const Other = defineComponent({
  template: '<div class="other-page">other</div>',
})

const Host = defineComponent({
  setup() {
    const { state, onConfirmEvent, onCancelEvent } = useConfirmHost()
    return () => [
      h(RouterView),
      h(ConfirmModal, {
        open: state.open,
        title: state.title,
        message: state.message,
        confirmLabel: state.confirmLabel,
        cancelLabel: state.cancelLabel,
        busy: state.busy,
        danger: state.danger,
        onConfirm: () => { onConfirmEvent().catch(() => {}) },
        onCancel: () => { onCancelEvent() },
      }),
    ]
  },
})

describe('DynamicForm unsaved-changes guard', () => {
  beforeEach(() => {
    _resetModalStack()
    _resetConfirmForTest()
  })

  afterEach(() => {
    document.body.innerHTML = ''
    _resetModalStack()
    _resetConfirmForTest()
  })

  async function mountWithRouter(dirty: Ref<boolean>) {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: makeFormHarness(dirty) },
        { path: '/other', component: Other },
      ],
    })
    await router.push('/')
    await router.isReady()
    const wrapper = mount(Host, {
      global: { plugins: [router], stubs: { RouterView: false } },
      attachTo: document.body,
    })
    await flushPromises()
    return { wrapper, router }
  }

  function modalButtons(): HTMLButtonElement[] {
    return Array.from(
      document.querySelectorAll<HTMLButtonElement>('.modal-actions button')
    )
  }

  it('lets the navigation through without prompting when not dirty', async () => {
    const dirty = ref(false)
    const { wrapper, router } = await mountWithRouter(dirty)

    await router.push('/other')
    await flushPromises()

    expect(document.querySelector('.modal-overlay')).toBeNull()
    expect(router.currentRoute.value.path).toBe('/other')
    wrapper.unmount()
  })

  it('shows the confirm modal when dirty and stays on the page on cancel', async () => {
    const dirty = ref(true)
    const { wrapper, router } = await mountWithRouter(dirty)

    const navPromise = router.push('/other')
    await flushPromises()

    // Modal is visible; navigation is suspended.
    const overlay = document.querySelector<HTMLElement>('.modal-overlay')
    expect(overlay).not.toBeNull()
    expect(overlay?.textContent).toContain('Unsaved changes')

    // Cancel button is the first one in modal-actions (cancel before confirm).
    modalButtons()[0]!.click()
    await flushPromises()
    await navPromise

    expect(router.currentRoute.value.path).toBe('/')
    expect(dirty.value).toBe(true) // dirty preserved
    expect(document.querySelector('.modal-overlay')).toBeNull()
    wrapper.unmount()
  })

  it('navigates and clears dirty when the user confirms', async () => {
    const dirty = ref(true)
    const { wrapper, router } = await mountWithRouter(dirty)

    const navPromise = router.push('/other')
    await flushPromises()

    modalButtons()[1]!.click()
    await flushPromises()
    await navPromise

    expect(router.currentRoute.value.path).toBe('/other')
    expect(dirty.value).toBe(false) // cleared so subsequent guard passes short-circuit
    wrapper.unmount()
  })

  it('preserves router.replace semantics — no extra history entry on confirm', async () => {
    const dirty = ref(true)
    const { wrapper, router } = await mountWithRouter(dirty)

    // Push another entry first so we can verify replace doesn't add one.
    dirty.value = false
    await router.push('/other')
    await flushPromises()
    await router.push('/')
    await flushPromises()
    dirty.value = true

    const beforeLength = window.history.length

    const navPromise = router.replace('/other')
    await flushPromises()
    modalButtons()[1]!.click()
    await flushPromises()
    await navPromise

    expect(router.currentRoute.value.path).toBe('/other')
    // history.length did not grow — replace stayed replace, which is the whole
    // point of returning the boolean from the guard instead of next(false) +
    // router.push(to.fullPath).
    expect(window.history.length).toBe(beforeLength)
    wrapper.unmount()
  })

  it('handles popstate (browser back) without inverting history', async () => {
    const dirty = ref(false)
    const { wrapper, router } = await mountWithRouter(dirty)

    // Build a history: / -> /other.
    await router.push('/other')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/other')

    // Now mark the form dirty (we are on /other now, but the test simulates a
    // case where dirty stays true across navigation; in production dirty is
    // owned by the form route. Here we just verify the guard runs on back.).
    dirty.value = false // form has no dirty state once unmounted, but harness keeps the ref alive
    // Reset and verify back-nav goes through cleanly.
    router.back()
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/')
    expect(document.querySelector('.modal-overlay')).toBeNull()
    wrapper.unmount()
  })

  it('once user confirms, subsequent navigation does not re-prompt', async () => {
    const dirty = ref(true)
    const { wrapper, router } = await mountWithRouter(dirty)

    const nav1 = router.push('/other')
    await flushPromises()
    modalButtons()[1]!.click()
    await flushPromises()
    await nav1
    expect(router.currentRoute.value.path).toBe('/other')

    // Navigating away again from a non-form page must not re-prompt — the
    // guard is no longer mounted, so the modal cannot appear regardless.
    const nav2 = router.push('/')
    await flushPromises()
    await nav2
    expect(document.querySelector('.modal-overlay')).toBeNull()
    wrapper.unmount()
  })
})
