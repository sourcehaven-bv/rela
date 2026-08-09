// Tests for the inline-create dialog (TKT-OMUD56).
//
// These exist because code review found three bugs that sat in the gap between
// the widget tests (which stub this modal) and the e2e suite (which had no
// keyboard or discard coverage):
//
//   1. Cmd+Enter was dead — the embedded form deliberately skips its own
//      document listener, and nothing here picked the shortcut up, so the
//      Create button advertised a `⌘↵` hint that did nothing.
//   2. The discard-confirm rendered UNDERNEATH this dialog (both overlays at
//      z-index 1000, both Teleported, later mount wins), so Escape on a dirty
//      form appeared to freeze.
//   3. Dirtiness was sniffed from bubbling input/change events, which miss
//      every non-native widget — a relation selection, a wizard step, a
//      CodeMirror body edit. A form with a written body read as pristine and
//      was discarded with no prompt.
//
// The fix for all three is that the modal ASKS the form (defineExpose) rather
// than inferring from the DOM. These tests pin that contract.

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import InlineCreateFormModal from './InlineCreateFormModal.vue'
import { useSchemaStore } from '@/stores'
import type { Entity } from '@/types'

const confirmMock = vi.fn()
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: confirmMock }),
}))

const CREATED: Entity = { id: 'FEAT-1', type: 'feature', properties: {} }

/** Stands in for DynamicForm, exposing the same handle the real one does. */
function formStub(state: { dirty?: boolean; saving?: boolean } = {}) {
  const submit = vi.fn()
  return {
    submit,
    component: {
      name: 'DynamicForm',
      props: ['formId', 'embedded'],
      emits: ['inline-created', 'inline-cancelled'],
      setup(_: unknown, { expose }: { expose: (o: object) => void }) {
        expose({
          isDirty: () => state.dirty ?? false,
          isSaving: () => state.saving ?? false,
          submit,
        })
        return () => null
      },
    },
  }
}

function mountModal(state: { dirty?: boolean; saving?: boolean } = {}) {
  const schema = useSchemaStore()
  schema.entityTypes.set('feature', { name: 'feature', label: 'Feature' } as never)

  const { submit, component } = formStub(state)
  const wrapper = mount(InlineCreateFormModal, {
    props: { show: true, formId: 'create_feature', entityType: 'feature' },
    global: { stubs: { DynamicForm: component } },
    attachTo: document.body,
  })
  return { wrapper, submit }
}

// The dialog is Teleported to body, so it lives outside the wrapper's tree.
function dispatchKey(key: string, mods: KeyboardEventInit = {}) {
  const el = document.querySelector('[role="dialog"]')
  if (!el) throw new Error('dialog not rendered')
  el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, ...mods }))
}

describe('InlineCreateFormModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    confirmMock.mockReset()
    document.body.replaceChildren()
  })

  it('submits the nested form on Cmd+Enter', async () => {
    const { wrapper, submit } = mountModal()

    dispatchKey('Enter', { metaKey: true })
    await flushPromises()

    expect(submit).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('submits on Ctrl+Enter too', async () => {
    const { wrapper, submit } = mountModal()

    dispatchKey('Enter', { ctrlKey: true })
    await flushPromises()

    expect(submit).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('does not submit on a bare Enter', async () => {
    // Enter inside a text field must not submit the whole form.
    const { wrapper, submit } = mountModal()

    dispatchKey('Enter')
    await flushPromises()

    expect(submit).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('closes on Escape without confirming when the form is pristine', async () => {
    const { wrapper } = mountModal({ dirty: false })

    dispatchKey('Escape')
    await flushPromises()

    expect(confirmMock).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('asks the FORM whether it is dirty rather than watching DOM events', async () => {
    // The regression: a body edit or relation selection leaves no bubbling
    // input event, so an event-sniffing modal would discard it silently.
    confirmMock.mockResolvedValue(true)
    const { wrapper } = mountModal({ dirty: true })

    dispatchKey('Escape')
    await flushPromises()

    expect(confirmMock).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('stays open when the discard is declined', async () => {
    confirmMock.mockResolvedValue(false)
    const { wrapper } = mountModal({ dirty: true })

    dispatchKey('Escape')
    await flushPromises()

    expect(wrapper.emitted('close')).toBeUndefined()
    wrapper.unmount()
  })

  it('refuses to close while a create is in flight', async () => {
    // Tearing the form out mid-POST would leave the user unsure whether the
    // entity was created.
    const { wrapper } = mountModal({ dirty: true, saving: true })

    dispatchKey('Escape')
    await flushPromises()

    expect(confirmMock).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toBeUndefined()
    wrapper.unmount()
  })

  it('forwards the created entity without a discard prompt', async () => {
    const { wrapper } = mountModal({ dirty: true })

    wrapper.findComponent({ name: 'DynamicForm' }).vm.$emit('inline-created', CREATED)
    await flushPromises()

    expect(wrapper.emitted('created')?.[0]?.[0]).toEqual(CREATED)
    expect(confirmMock).not.toHaveBeenCalled()
    wrapper.unmount()
  })

})
